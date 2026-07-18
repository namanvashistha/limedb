"""Typed result objects returned by the SDK.

These mirror the JSON shapes emitted by ``internal/server/server.go`` and
``internal/node/service.go``. Only the fields the SDK exposes are modelled;
the raw dict is kept on ``.raw`` so nothing is lost.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class GetResult:
    """Full result of a GET, including LWW metadata."""

    value: str
    timestamp_micros: int
    node_url: str
    raw: dict[str, Any] = field(default_factory=dict, repr=False)

    @classmethod
    def from_json(cls, data: dict[str, Any]) -> "GetResult":
        return cls(
            value=data.get("value", ""),
            timestamp_micros=int(data.get("timestamp_micros", 0)),
            node_url=data.get("nodeUrl", ""),
            raw=data,
        )


@dataclass(frozen=True)
class KeyEntry:
    key: str
    value: str
    size: int


@dataclass(frozen=True)
class KeyPage:
    """One page of a node's local keys (``GET /api/v1/keys``)."""

    keys: list[KeyEntry]
    total: int
    page: int
    page_size: int
    total_pages: int
    raw: dict[str, Any] = field(default_factory=dict, repr=False)

    @classmethod
    def from_json(cls, data: dict[str, Any]) -> "KeyPage":
        entries = [
            KeyEntry(key=k.get("key", ""), value=k.get("value", ""), size=int(k.get("size", 0)))
            for k in data.get("keys", []) or []
        ]
        return cls(
            keys=entries,
            total=int(data.get("total", 0)),
            page=int(data.get("page", 0)),
            page_size=int(data.get("pageSize", 0)),
            total_pages=int(data.get("totalPages", 0)),
            raw=data,
        )


@dataclass(frozen=True)
class ReplicaNode:
    node_url: str
    is_primary: bool
    is_local: bool
    has_value: bool


@dataclass(frozen=True)
class ReplicaInfo:
    """Replica placement for a key (``GET /api/v1/replicas/{key}``)."""

    key: str
    replication_factor: int
    quorum: int
    replicas: list[ReplicaNode]
    raw: dict[str, Any] = field(default_factory=dict, repr=False)

    @classmethod
    def from_json(cls, data: dict[str, Any]) -> "ReplicaInfo":
        nodes = [
            ReplicaNode(
                node_url=r.get("node_url", ""),
                is_primary=bool(r.get("is_primary", False)),
                is_local=bool(r.get("is_local", False)),
                has_value=bool(r.get("has_value", False)),
            )
            for r in data.get("replicas", []) or []
        ]
        return cls(
            key=data.get("key", ""),
            replication_factor=int(data.get("replication_factor", 0)),
            quorum=int(data.get("quorum", 0)),
            replicas=nodes,
            raw=data,
        )
