#!/usr/bin/env python3
"""HULK — a load/stress tester for LimeDB, built on the limedb SDK.

Every data operation goes through the SDK's ``AsyncClient`` (set / get /
delete); only the underlying httpx connection pool is tuned directly so the
client can drive high concurrency.

Examples
--------
    # 200k ops, 128 concurrent workers, 80% reads / 20% writes
    python hulk.py --urls http://localhost:8484 --ops 200000 --concurrency 128

    # run for 30 seconds instead of a fixed op count, across a 4-node cluster
    python hulk.py \
        --urls http://localhost:8484,http://localhost:8485 \
        --duration 30 --concurrency 256 --mix 70/25/5

    # pure write flood
    python hulk.py --ops 100000 --mix 0/100/0

The mix is read/write/delete percentages (must sum to 100).
"""

from __future__ import annotations

import argparse
import asyncio
import random
import signal
import statistics
import string
import sys
import time
from dataclasses import dataclass, field

import httpx

# Allow running straight from the repo without installing the SDK.
try:
    from limedb import AsyncClient, KeyNotFound, LimeDBError
except ImportError:  # pragma: no cover - convenience for in-repo runs
    import pathlib

    sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1] / "src"))
    from limedb import AsyncClient, KeyNotFound, LimeDBError


# --------------------------------------------------------------------------- stats


@dataclass
class OpStats:
    latencies_us: list[float] = field(default_factory=list)
    errors: int = 0

    def record(self, latency_us: float) -> None:
        self.latencies_us.append(latency_us)

    @property
    def count(self) -> int:
        return len(self.latencies_us)


@dataclass
class Metrics:
    read: OpStats = field(default_factory=OpStats)
    write: OpStats = field(default_factory=OpStats)
    delete: OpStats = field(default_factory=OpStats)
    error_samples: list[str] = field(default_factory=list)

    def total_ops(self) -> int:
        return self.read.count + self.write.count + self.delete.count

    def total_errors(self) -> int:
        return self.read.errors + self.write.errors + self.delete.errors

    def note_error(self, exc: Exception) -> None:
        if len(self.error_samples) < 5:
            self.error_samples.append(f"{type(exc).__name__}: {exc}")


def percentiles(values_us: list[float]) -> dict[str, float]:
    if not values_us:
        return {}
    s = sorted(values_us)
    n = len(s)

    def q(p: float) -> float:
        idx = min(n - 1, int(round((p / 100.0) * (n - 1))))
        return s[idx]

    return {
        "min": s[0] / 1000.0,
        "p50": q(50) / 1000.0,
        "p90": q(90) / 1000.0,
        "p95": q(95) / 1000.0,
        "p99": q(99) / 1000.0,
        "max": s[-1] / 1000.0,
        "mean": statistics.fmean(s) / 1000.0,
    }


# --------------------------------------------------------------------------- worker


class Hulk:
    def __init__(self, args: argparse.Namespace):
        self.args = args
        self.metrics = Metrics()
        self.value = _random_value(args.value_size)
        self.keyspace = args.keyspace
        r, w, d = args.mix
        # cumulative thresholds for a single [0,100) draw
        self._read_thresh = r
        self._write_thresh = r + w
        self._deadline: float | None = None
        self._remaining_ops: int | None = None
        self._lock = asyncio.Lock()
        self._done = 0
        self._stop = False  # set by SIGINT → workers drain and run() returns

    def _key(self) -> str:
        return f"{self.args.prefix}{random.randrange(self.keyspace)}"

    async def _claim_op(self) -> bool:
        """Return True if this worker should perform one more op."""
        if self._stop:
            return False
        if self._deadline is not None:
            return time.monotonic() < self._deadline
        async with self._lock:
            if self._remaining_ops is not None and self._remaining_ops <= 0:
                return False
            if self._remaining_ops is not None:
                self._remaining_ops -= 1
            return True

    async def _one_op(self, db: AsyncClient) -> None:
        roll = random.uniform(0, 100)
        key = self._key()
        start = time.perf_counter()
        try:
            if roll < self._read_thresh:
                await db.get_or_none(key)
                op = self.metrics.read
            elif roll < self._write_thresh:
                await db.set(key, self.value)
                op = self.metrics.write
            else:
                await db.delete(key)
                op = self.metrics.delete
        except (LimeDBError, httpx.HTTPError) as exc:
            # attribute the error to the op type we attempted
            if roll < self._read_thresh:
                self.metrics.read.errors += 1
            elif roll < self._write_thresh:
                self.metrics.write.errors += 1
            else:
                self.metrics.delete.errors += 1
            self.metrics.note_error(exc)
            self._done += 1
            return
        op.record((time.perf_counter() - start) * 1_000_000)
        self._done += 1

    async def _worker(self, db: AsyncClient) -> None:
        while await self._claim_op():
            await self._one_op(db)

    async def _reporter(self, start: float) -> None:
        last_done = 0
        last_t = start
        while True:
            await asyncio.sleep(1.0)
            now = time.monotonic()
            done = self._done
            rps = (done - last_done) / (now - last_t) if now > last_t else 0.0
            elapsed = now - start
            sys.stderr.write(
                f"\r  {elapsed:5.1f}s  ops={done:>9,}  "
                f"rps={rps:>10,.0f}  errors={self.metrics.total_errors():>6,}   "
            )
            sys.stderr.flush()
            last_done, last_t = done, now

    def _make_clients(self, http: httpx.AsyncClient) -> list[AsyncClient]:
        """One SDK client per seed, each pinned to a different node as its
        primary ingress (via a rotated seed list) so load spreads across the
        cluster while every client keeps full failover to the other nodes."""
        seeds = self.args.urls
        return [
            AsyncClient(seeds[i:] + seeds[:i], client=http)
            for i in range(len(seeds))
        ]

    def _install_sigint(self) -> "callable | None":
        """First Ctrl-C → drain in-flight ops and print the report.
        Second Ctrl-C → abort immediately. Returns a cleanup callable."""
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            return None
        hits = 0

        def handler() -> None:
            nonlocal hits
            hits += 1
            if hits == 1:
                self._stop = True
                sys.stderr.write(
                    "\n  ^C — draining in-flight ops, printing report "
                    "(press Ctrl-C again to abort)...\n"
                )
            else:
                sys.stderr.write("\n  ^C^C — aborting now\n")
                for t in asyncio.all_tasks(loop):
                    t.cancel()

        try:
            loop.add_signal_handler(signal.SIGINT, handler)
        except NotImplementedError:  # e.g. Windows proactor loop
            return None
        return lambda: loop.remove_signal_handler(signal.SIGINT)

    async def run(self) -> float:
        limits = httpx.Limits(
            max_connections=self.args.concurrency * 2,
            max_keepalive_connections=self.args.concurrency,
        )
        http = httpx.AsyncClient(timeout=self.args.timeout, limits=limits)
        clients = self._make_clients(http)
        remove_sigint = self._install_sigint()

        if (
            self.args.warmup
            and not self._stop
            and (self._read_thresh > 0 or self._write_thresh < 100)
        ):
            await self._warmup(clients)

        if self.args.duration:
            self._deadline = time.monotonic() + self.args.duration
        else:
            self._remaining_ops = self.args.ops

        start = time.monotonic()
        reporter = asyncio.create_task(self._reporter(start))
        try:
            # round-robin workers across the pinned clients → spreads ingress
            await asyncio.gather(
                *(
                    self._worker(clients[i % len(clients)])
                    for i in range(self.args.concurrency)
                )
            )
        except asyncio.CancelledError:
            self._stop = True  # second Ctrl-C: fall through to the report
        finally:
            reporter.cancel()
            await asyncio.gather(reporter, return_exceptions=True)
            if remove_sigint is not None:
                remove_sigint()
            await http.aclose()
        elapsed = time.monotonic() - start
        sys.stderr.write("\n")
        return elapsed

    async def _warmup(self, clients: list[AsyncClient]) -> None:
        """Pre-populate the keyspace so reads/deletes hit existing keys."""
        sys.stderr.write(f"  warming up {self.keyspace:,} keys...\n")
        sem = asyncio.Semaphore(self.args.concurrency)

        async def put(i: int) -> None:
            if self._stop:
                return
            async with sem:
                if self._stop:
                    return
                try:
                    await clients[i % len(clients)].set(f"{self.args.prefix}{i}", self.value)
                except (LimeDBError, httpx.HTTPError):
                    pass

        await asyncio.gather(*(put(i) for i in range(self.keyspace)))


def _random_value(size: int) -> str:
    alphabet = string.ascii_letters + string.digits
    return "".join(random.choices(alphabet, k=size))


# --------------------------------------------------------------------------- report


def print_report(args: argparse.Namespace, m: Metrics, elapsed: float) -> None:
    total = m.total_ops()
    rps = total / elapsed if elapsed > 0 else 0.0
    print("\n" + "=" * 60)
    print("HULK LOAD TEST — RESULTS")
    print("=" * 60)
    print(f"  targets       : {', '.join(args.urls)}")
    print(f"  concurrency   : {args.concurrency}")
    print(f"  mix (r/w/d)   : {args.mix[0]}/{args.mix[1]}/{args.mix[2]}")
    print(f"  keyspace      : {args.keyspace:,}   value size: {args.value_size} B")
    print(f"  duration      : {elapsed:.2f} s")
    print(f"  total ops     : {total:,}")
    print(f"  throughput    : {rps:,.0f} ops/s")
    errs = m.total_errors()
    err_rate = (errs / (total + errs) * 100) if (total + errs) else 0.0
    print(f"  errors        : {errs:,}  ({err_rate:.2f}%)")
    print("-" * 60)
    print(f"  {'op':<8}{'count':>12}{'p50':>9}{'p90':>9}{'p99':>9}{'max':>9}  (ms)")
    for name, op in (("read", m.read), ("write", m.write), ("delete", m.delete)):
        if op.count == 0 and op.errors == 0:
            continue
        p = percentiles(op.latencies_us)
        if p:
            print(
                f"  {name:<8}{op.count:>12,}{p['p50']:>9.2f}{p['p90']:>9.2f}"
                f"{p['p99']:>9.2f}{p['max']:>9.2f}"
            )
        else:
            print(f"  {name:<8}{op.count:>12,}{'—':>9}{'—':>9}{'—':>9}{'—':>9}")
    if m.error_samples:
        print("-" * 60)
        print("  error samples:")
        for s in m.error_samples:
            print(f"    - {s}")
    print("=" * 60)


# --------------------------------------------------------------------------- cli


def parse_mix(raw: str) -> tuple[int, int, int]:
    parts = raw.split("/")
    if len(parts) != 3:
        raise argparse.ArgumentTypeError("mix must be read/write/delete, e.g. 80/20/0")
    try:
        r, w, d = (int(p) for p in parts)
    except ValueError:
        raise argparse.ArgumentTypeError("mix values must be integers")
    if r + w + d != 100:
        raise argparse.ArgumentTypeError(f"mix must sum to 100 (got {r + w + d})")
    return r, w, d


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="HULK — LimeDB load/stress tester (built on the limedb SDK).",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    p.add_argument(
        "--urls",
        default="http://localhost:8484",
        help="comma-separated seed node URLs",
    )
    stop = p.add_mutually_exclusive_group()
    stop.add_argument("--ops", type=int, default=100_000, help="total operations to run")
    stop.add_argument(
        "--duration", type=float, default=None, help="run for N seconds instead of --ops"
    )
    p.add_argument("--concurrency", type=int, default=64, help="concurrent workers")
    p.add_argument(
        "--mix", type=parse_mix, default=(80, 20, 0), help="read/write/delete pct, sums to 100"
    )
    p.add_argument("--keyspace", type=int, default=10_000, help="number of distinct keys")
    p.add_argument("--value-size", type=int, default=64, help="value size in bytes")
    p.add_argument("--prefix", default="hulk:", help="key prefix")
    p.add_argument("--timeout", type=float, default=10.0, help="per-request timeout (s)")
    p.add_argument(
        "--warmup",
        action="store_true",
        help="pre-populate the keyspace before the run (recommended for read-heavy mixes)",
    )
    args = p.parse_args(argv)
    args.urls = [u.strip() for u in args.urls.split(",") if u.strip()]
    if not args.urls:
        p.error("at least one URL is required")
    if args.concurrency < 1:
        p.error("--concurrency must be >= 1")
    return args


async def _amain(args: argparse.Namespace) -> int:
    hulk = Hulk(args)
    mode = f"{args.duration}s" if args.duration else f"{args.ops:,} ops"
    print(
        f"HULK SMASH  →  {mode} · {args.concurrency} workers · "
        f"mix {args.mix[0]}/{args.mix[1]}/{args.mix[2]} · {', '.join(args.urls)}",
        file=sys.stderr,
    )
    try:
        elapsed = await hulk.run()
    except LimeDBError as exc:
        print(f"\nfatal: cluster unreachable: {exc}", file=sys.stderr)
        return 1
    print_report(args, hulk.metrics, elapsed)
    return 0


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        return asyncio.run(_amain(args))
    except KeyboardInterrupt:
        print("\ninterrupted", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
