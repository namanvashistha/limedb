"""
hulk.py — LimeDB load tester
Tests SET throughput, GET latency, quorum durability, and mixed read/write ratio.

Usage:
    uv run hulk.py                                              # hits prod, discovers nodes via gossip
    uv run hulk.py https://limedb.namanvashistha.com/api/proxy  # same, explicit
    uv run hulk.py http://localhost:7001/api/v1 --requests 5000 --concurrency 50
    uv run hulk.py http://localhost:7001/api/v1 --mode mixed --read-ratio 0.7
    uv run hulk.py http://localhost:7001/api/v1 --mode verify  # write then read back and verify values

Options:
    base_url        Base API URL (default: https://limedb.namanvashistha.com/api/proxy)
    --requests      Total number of requests (default: 1000)
    --concurrency   Concurrent workers (default: 20)
    --mode          write | read | mixed | verify (default: write)
    --read-ratio    Fraction of reads in mixed mode (default: 0.5)
    --value-size    Approximate value size in bytes (default: 64)
    --node          Pin to a single node URL; skips auto-discovery
    --timeout       Per-request timeout in seconds (default: 5)

Node routing:
    By default hulk fetches /api/v1/cluster/gossip (or /cluster/gossip for proxy endpoints),
    collects all active+stale node URLs, and picks one at random for every request.
    Pass --node <url> to override and pin to a single node.
"""

# /// script
# dependencies = ["httpx", "rich"]
# ///

import argparse
import random
import string
import time
from collections import defaultdict, deque
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from threading import Lock, Event
from typing import Optional

import httpx
from rich.console import Console
from rich.live import Live
from rich.panel import Panel
from rich.progress import BarColumn, Progress, SpinnerColumn, TaskID, TextColumn, TimeElapsedColumn
from rich.table import Table
from rich.text import Text

console = Console()

ADJECTIVES = ["quick", "lazy", "angry", "happy", "silent", "bold", "frugal", "cosmic", "neon", "rusty"]
ANIMALS = ["parrot", "panda", "jaguar", "cobra", "falcon", "narwhal", "sloth", "mantis", "orca", "gecko"]


def random_key(i: int) -> str:
    return f"{random.choice(ADJECTIVES)}-{random.choice(ANIMALS)}-{i}"


def random_value(size: int) -> str:
    return "".join(random.choices(string.ascii_letters + string.digits + " ", k=size))


@dataclass
class Stats:
    lock: Lock = field(default_factory=Lock)
    stop_event: Event = field(default_factory=Event)
    recent_latencies: deque = field(default_factory=lambda: deque(maxlen=100))
    total: int = 0
    success: int = 0
    errors: int = 0
    quorum_failures: int = 0
    latencies: list = field(default_factory=list)
    status_codes: dict = field(default_factory=lambda: defaultdict(int))
    verified_ok: int = 0
    verified_fail: int = 0

    def record(self, latency_ms: float, status: int, body: str = ""):
        with self.lock:
            self.total += 1
            self.latencies.append(latency_ms)
            self.recent_latencies.append(latency_ms)
            self.status_codes[status] += 1
            if 200 <= status < 300:
                self.success += 1
            else:
                self.errors += 1
                if "quorum" in body.lower():
                    self.quorum_failures += 1

    def percentile(self, p: float) -> float:
        if not self.latencies:
            return 0.0
        s = sorted(self.latencies)
        idx = int(len(s) * p / 100)
        return s[min(idx, len(s) - 1)]

    def summary(self) -> dict:
        with self.lock:
            lats = sorted(self.latencies)
            total = len(lats)
            return {
                "total": self.total,
                "success": self.success,
                "errors": self.errors,
                "quorum_failures": self.quorum_failures,
                "verified_ok": self.verified_ok,
                "verified_fail": self.verified_fail,
                "p50": self.percentile(50),
                "p95": self.percentile(95),
                "p99": self.percentile(99),
                "max": max(lats) if lats else 0,
                "min": min(lats) if lats else 0,
                "avg": sum(lats) / total if total else 0,
                "status_codes": dict(self.status_codes),
            }


PROD_URL = "https://limedb.namanvashistha.com/api/proxy"


def fetch_nodes(base_url: str, timeout: float) -> list[str]:
    """Discover active/stale cluster nodes via the gossip endpoint.

    Works for both proxy URLs  (/api/proxy -> /cluster/gossip?node=…)
    and direct node URLs       (/api/v1   -> /api/v1/cluster/gossip).
    Returns a list of node URLs (including self); falls back to [] on error.
    """
    gossip_url = base_url.rstrip("/") + "/cluster/gossip"

    try:
        with httpx.Client() as client:
            r = client.get(gossip_url, timeout=timeout)
            r.raise_for_status()
            data = r.json()
    except Exception as e:
        console.print(f"[yellow]Warning: could not fetch gossip ({e}); node routing disabled[/yellow]")
        return []

    nodes: list[str] = []
    self_url = data.get("node_url", "")
    if self_url:
        nodes.append(self_url)
    for peer in data.get("peer_details", []):
        if peer.get("status") in ("active", "stale") and peer.get("url"):
            nodes.append(peer["url"])
    return nodes


def pick_node(nodes: list[str]) -> Optional[str]:
    """Return a random node URL, or None if the list is empty."""
    return random.choice(nodes) if nodes else None


def build_url(base: str, path: str, node: Optional[str]) -> str:
    """Build the full URL, appending ?node= if using proxy."""
    url = f"{base}/{path.lstrip('/')}"
    if node:
        sep = "&" if "?" in url else "?"
        url += f"{sep}node={node}"
    return url


def do_set(client: httpx.Client, base: str, key: str, value: str, nodes: list[str], timeout: float) -> tuple[float, int, str]:
    url = build_url(base, "set", pick_node(nodes))
    start = time.perf_counter()
    try:
        r = client.post(url, json={"key": key, "value": value}, timeout=timeout)
        ms = (time.perf_counter() - start) * 1000
        return ms, r.status_code, r.text
    except Exception as e:
        ms = (time.perf_counter() - start) * 1000
        return ms, 0, str(e)


def do_get(client: httpx.Client, base: str, key: str, nodes: list[str], timeout: float) -> tuple[float, int, str]:
    url = build_url(base, f"get/{key}", pick_node(nodes))
    start = time.perf_counter()
    try:
        r = client.get(url, timeout=timeout)
        ms = (time.perf_counter() - start) * 1000
        return ms, r.status_code, r.text
    except Exception as e:
        ms = (time.perf_counter() - start) * 1000
        return ms, 0, str(e)


def run_write(args, stats: Stats, keys: list[tuple[str, str]], progress: Progress, task: TaskID):
    with httpx.Client(http2=False) as client:
        for key, value in keys:
            if stats.stop_event.is_set():
                break
            ms, status, body = do_set(client, args.base_url, key, value, args.nodes, args.timeout)
            stats.record(ms, status, body)
            progress.advance(task)


def run_read(args, stats: Stats, keys: list[str], progress: Progress, task: TaskID):
    with httpx.Client(http2=False) as client:
        for key in keys:
            if stats.stop_event.is_set():
                break
            ms, status, body = do_get(client, args.base_url, key, args.nodes, args.timeout)
            stats.record(ms, status, body)
            progress.advance(task)


def run_mixed(args, stats: Stats, keys: list[tuple[str, str]], progress: Progress, task: TaskID):
    read_keys: list[str] = []
    with httpx.Client(http2=False) as client:
        for key, value in keys:
            if stats.stop_event.is_set():
                break
            if read_keys and random.random() < args.read_ratio:
                rk = random.choice(read_keys)
                ms, status, body = do_get(client, args.base_url, rk, args.nodes, args.timeout)
            else:
                ms, status, body = do_set(client, args.base_url, key, value, args.nodes, args.timeout)
                if 200 <= status < 300:
                    read_keys.append(key)
            stats.record(ms, status, body)
            progress.advance(task)


def run_verify(args, stats: Stats, keys: list[tuple[str, str]], progress: Progress, task: TaskID):
    """Write a key, then immediately read it back and verify the value matches."""
    with httpx.Client(http2=False) as client:
        for key, value in keys:
            if stats.stop_event.is_set():
                break
            ms, status, body = do_set(client, args.base_url, key, value, args.nodes, args.timeout)
            stats.record(ms, status, body)
            progress.advance(task)

            if 200 <= status < 300:
                ms2, status2, body2 = do_get(client, args.base_url, key, args.nodes, args.timeout)
                stats.record(ms2, status2, body2)
                with stats.lock:
                    if status2 == 200 and value in body2:
                        stats.verified_ok += 1
                    else:
                        stats.verified_fail += 1
            progress.advance(task)


def get_sparkline(values: list[float]) -> str:
    if not values:
        return ""
    bars = " ▂▃▄▅▆▇█"
    min_val = min(values)
    max_val = max(values)
    span = max_val - min_val
    if span == 0:
        return bars[0] * len(values)
    res = []
    for v in values:
        idx = int((v - min_val) / span * (len(bars) - 1))
        res.append(bars[idx])
    return "".join(res)


class Dashboard:
    def __init__(self, progress, stats, start_time):
        self.progress = progress
        self.stats = stats
        self.start_time = start_time

    def __rich__(self):
        elapsed = time.perf_counter() - self.start_time
        with self.stats.lock:
            total = self.stats.total
            success = self.stats.success
            errors = self.stats.errors
            recent = list(self.stats.recent_latencies)
            
        rps = total / elapsed if elapsed > 0 else 0
        spark = get_sparkline(recent)
        
        table = Table.grid(expand=True)
        table.add_row(self.progress)
        table.add_row("")
        avg_recent = sum(recent)/len(recent) if recent else 0
        stats_text = (
            f"[cyan]RPS:[/] [bold]{rps:.1f}[/]  |  "
            f"[green]OK:[/] {success}  |  "
            f"[red]Fail:[/] {errors}  |  "
            f"[yellow]Avg (last {len(recent)}):[/] {avg_recent:.1f} ms"
        )
        table.add_row(stats_text)
        if spark:
             table.add_row(f"[magenta]Latency Map:[/] {spark}")
        return table


def print_results(stats: Stats, elapsed: float, mode: str):
    s = stats.summary()
    rps = s["total"] / elapsed if elapsed > 0 else 0
    success_rate = s["success"] / s["total"] * 100 if s["total"] else 0

    table = Table(title=f"[bold]hulk results — {mode} mode[/bold]", show_header=True, header_style="bold cyan")
    table.add_column("Metric", style="dim")
    table.add_column("Value", justify="right")

    table.add_row("Total requests", str(s["total"]))
    table.add_row("Success", f"[green]{s['success']}[/green]")
    table.add_row("Errors", f"[red]{s['errors']}[/red]" if s["errors"] else "0")
    table.add_row("Quorum failures", f"[red]{s['quorum_failures']}[/red]" if s["quorum_failures"] else "0")

    if mode == "verify":
        table.add_row("Verify OK", f"[green]{s['verified_ok']}[/green]")
        table.add_row("Verify FAIL", f"[red]{s['verified_fail']}[/red]" if s["verified_fail"] else "0")

    table.add_row("", "")
    table.add_row("Throughput", f"[bold]{rps:.1f} req/s[/bold]")
    table.add_row("Success rate", f"{'[green]' if success_rate == 100 else '[yellow]'}{success_rate:.1f}%[/]")
    table.add_row("Wall time", f"{elapsed:.2f}s")
    table.add_row("", "")
    table.add_row("p50 latency", f"{s['p50']:.1f} ms")
    table.add_row("p95 latency", f"{s['p95']:.1f} ms")
    table.add_row("p99 latency", f"[yellow]{s['p99']:.1f} ms[/yellow]")
    table.add_row("max latency", f"[red]{s['max']:.1f} ms[/red]")
    table.add_row("avg latency", f"{s['avg']:.1f} ms")
    table.add_row("", "")

    for code, count in sorted(s["status_codes"].items()):
        color = "green" if 200 <= code < 300 else "red"
        table.add_row(f"HTTP {code}", f"[{color}]{count}[/{color}]")

    console.print(table)


def main():
    parser = argparse.ArgumentParser(description="hulk — LimeDB load tester")
    parser.add_argument("base_url", nargs="?", default=PROD_URL, help="Base API URL (default: prod)")
    parser.add_argument("--requests", type=int, default=100000, help="Total requests")
    parser.add_argument("--concurrency", type=int, default=100, help="Concurrent workers")
    parser.add_argument("--mode", choices=["write", "read", "mixed", "verify"], default="write")
    parser.add_argument("--read-ratio", type=float, default=0.5, help="Read fraction in mixed mode")
    parser.add_argument("--value-size", type=int, default=10000, help="Value size in bytes")
    parser.add_argument("--node", type=str, default=None, help="Pin to one node URL; skips auto-discovery")
    parser.add_argument("--timeout", type=float, default=5.0, help="Per-request timeout (s)")
    args = parser.parse_args()

    n = args.requests
    c = args.concurrency

    # Resolve nodes: pinned > discovered > none
    if args.node:
        args.nodes = [args.node]
        node_display = f"[yellow]{args.node}[/yellow] (pinned)"
    else:
        console.print("[dim]Discovering cluster nodes via gossip…[/dim]")
        args.nodes = fetch_nodes(args.base_url, args.timeout)
        if args.nodes:
            for n_url in args.nodes:
                console.print(f"  [green]✓[/green] {n_url}")
            node_display = "[green]" + ", ".join(args.nodes) + "[/green]"
        else:
            args.nodes = []  # build_url will send without ?node=
            node_display = "[dim]none (no ?node= routing)[/dim]"

    console.print(Panel(
        f"[bold cyan]hulk[/bold cyan] → [white]{args.base_url}[/white]\n"
        f"mode=[yellow]{args.mode}[/yellow]  "
        f"requests=[yellow]{n}[/yellow]  "
        f"concurrency=[yellow]{c}[/yellow]  "
        f"value_size=[yellow]{args.value_size}B[/yellow]\n"
        f"nodes={node_display}",
        title="LimeDB Load Test",
        border_style="cyan",
    ))

    # Pre-generate all keys+values
    all_pairs = [(random_key(i), random_value(args.value_size)) for i in range(n)]

    # For read mode: first write all keys, then read them
    if args.mode == "read":
        console.print("[dim]Seeding keys before read test…[/dim]")
        with httpx.Client() as client:
            for key, value in all_pairs:
                do_set(client, args.base_url, key, value, args.nodes, args.timeout)

    # Split work across workers
    chunk_size = max(1, n // c)
    chunks = [all_pairs[i:i + chunk_size] for i in range(0, n, chunk_size)]

    stats = Stats()
    start_time = time.perf_counter()

    progress = Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TextColumn("{task.completed}/{task.total}"),
        TimeElapsedColumn(),
        console=console,
    )

    total_ops = n * 2 if args.mode == "verify" else n
    task = progress.add_task(f"[cyan]{args.mode}…", total=total_ops)

    worker_fn = {
        "write": run_write,
        "read": run_read,
        "mixed": run_mixed,
        "verify": run_verify,
    }[args.mode]

    dashboard = Dashboard(progress, stats, start_time)
    with Live(dashboard, console=console, refresh_per_second=10):
        with ThreadPoolExecutor(max_workers=c) as executor:
            try:
                futures = [executor.submit(worker_fn, args, stats, chunk, progress, task) for chunk in chunks]
                for f in as_completed(futures):
                    f.result()
            except KeyboardInterrupt:
                stats.stop_event.set()
                for f in futures:
                    f.cancel()
                console.print("\n[yellow]Interrupted! Stopping workers...[/yellow]")

    elapsed = time.perf_counter() - start_time
    print_results(stats, elapsed, args.mode)


if __name__ == "__main__":
    main()

