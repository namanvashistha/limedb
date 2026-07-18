"""LimeDB Python SDK.

A thin, typed client for the LimeDB distributed key-value store. Talk to any
node; requests are routed to the correct ring owner internally.

    from limedb import Client

    with Client("http://localhost:8484") as db:
        db.set("k", "v")
        print(db.get("k"))
"""

from __future__ import annotations

from .aclient import AsyncClient
from .client import Client
from .errors import (
    ClusterUnavailable,
    KeyNotFound,
    LimeDBError,
    RequestError,
)
from .models import (
    GetResult,
    KeyEntry,
    KeyPage,
    ReplicaInfo,
    ReplicaNode,
)

__version__ = "0.1.0"

__all__ = [
    "Client",
    "AsyncClient",
    "LimeDBError",
    "KeyNotFound",
    "RequestError",
    "ClusterUnavailable",
    "GetResult",
    "KeyEntry",
    "KeyPage",
    "ReplicaInfo",
    "ReplicaNode",
    "__version__",
]
