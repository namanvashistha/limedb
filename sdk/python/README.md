# LimeDB Python SDK

A thin, typed Python client for [LimeDB](../../README.md) — a distributed
key-value store. Any node can service any key (requests are routed to the ring
owner internally), so you only need one or more *seed* URLs. If a seed is
unreachable, the client rotates to the next one automatically.

## Install

From the repo (until published to PyPI):

```bash
pip install ./sdk/python
# or, editable, for development:
pip install -e "./sdk/python[test]"
```

Requires Python 3.9+ and [`httpx`](https://www.python-httpx.org/).

## Sync usage

```python
from limedb import Client

with Client("http://localhost:8484") as db:
    db.set("greeting", "hello")

    db.get("greeting")            # -> "hello"       (raises KeyNotFound if absent)
    db.get_or_none("missing")     # -> None
    db.delete("greeting")         # -> True if it existed, else False

    # dict-style sugar
    db["k"] = "v"
    print(db["k"])                # "v"
    print("k" in db)              # True

    # LWW metadata
    r = db.get_full("k")
    print(r.value, r.timestamp_micros, r.node_url)
```

Multiple seeds for failover:

```python
db = Client(["http://node1:8484", "http://node2:8484", "http://node3:8484"])
```

## Async usage

```python
import asyncio
from limedb import AsyncClient

async def main():
    async with AsyncClient("http://localhost:8484") as db:
        await db.set("k", "v")
        print(await db.get("k"))

asyncio.run(main())
```

The async API mirrors the sync one method-for-method; every network call is a
coroutine.

## Values are strings

The server stores string values. `set`/`get` pass strings through unchanged.
For structured data, use the explicit JSON helpers (no hidden magic):

```python
db.set_json("user:1", {"name": "ada", "age": 36})
db.get_json("user:1")   # -> {"name": "ada", "age": 36}
```

## Cluster inspection

```python
db.list_keys(page=1, page_size=20)   # KeyPage — the responding node's local shard
db.replicas("k")                     # ReplicaInfo — placement + who holds the key
db.health()                          # dict
db.cluster_state()                   # dict
db.ring_state()                      # dict
db.gossip_metrics()                  # dict
```

> `list_keys` reflects a single node's local shard, not the whole keyspace.

## Errors

| Exception | When |
|---|---|
| `KeyNotFound` | `get`/`get_full`/`get_json` on a missing key (subclasses `KeyError`) |
| `RequestError` | server returned a non-2xx status (has `.status_code`, `.body`) |
| `ClusterUnavailable` | every seed failed to connect |
| `LimeDBError` | base class for all of the above |

## Testing

```bash
cd sdk/python
pip install -e ".[test]"
pytest
```

Unit tests mock HTTP with [`respx`](https://lundberg.github.io/respx/) — no
running cluster needed.
