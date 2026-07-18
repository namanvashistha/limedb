"""Synchronous LimeDB client."""

from __future__ import annotations

import json as _json
from typing import Any, Iterable

import httpx

from . import _common as c
from .errors import ClusterUnavailable, KeyNotFound
from .models import GetResult, KeyPage, ReplicaInfo


class Client:
    """A blocking client for a LimeDB cluster.

    Any node can service any key (requests are routed to the ring owner
    internally), so you only need to supply one or more *seed* URLs. If a
    request fails to connect, the client rotates to the next seed and retries;
    a successful seed is remembered and tried first next time.

    Example::

        with Client("http://localhost:8484") as db:
            db.set("greeting", "hello")
            print(db.get("greeting"))          # "hello"
            print(db.get_or_none("missing"))   # None

    Values are strings in and strings out, matching the server. Use
    :meth:`set_json` / :meth:`get_json` when you want transparent JSON
    (de)serialisation.
    """

    def __init__(
        self,
        urls: str | Iterable[str],
        *,
        timeout: float = c.DEFAULT_TIMEOUT,
        client: httpx.Client | None = None,
    ):
        self._seeds = c.normalize_seeds(list(urls) if not isinstance(urls, str) else urls)
        self._preferred = 0  # index of the last seed that worked
        self._owns_client = client is None
        self._http = client or httpx.Client(timeout=timeout)

    # -- lifecycle ----------------------------------------------------------

    def close(self) -> None:
        if self._owns_client:
            self._http.close()

    def __enter__(self) -> "Client":
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

    # -- request plumbing ---------------------------------------------------

    def _ordered_seeds(self) -> list[str]:
        p = self._preferred
        return self._seeds[p:] + self._seeds[:p]

    def _request(self, method: str, path: str, **kw: Any) -> tuple[int, str]:
        """Try each seed in turn until one answers; return (status, text)."""
        tried: list[str] = []
        last_err: Exception | None = None
        for offset, base in enumerate(self._ordered_seeds()):
            url = f"{base}{path}"
            tried.append(base)
            try:
                resp = self._http.request(method, url, **kw)
            except httpx.TransportError as exc:  # connect/timeout/read errors
                last_err = exc
                continue
            # reachable node: remember it and return, even on HTTP error status
            self._preferred = (self._preferred + offset) % len(self._seeds)
            return resp.status_code, resp.text
        raise ClusterUnavailable(tried, last_err)

    # -- key/value ----------------------------------------------------------

    def get_full(self, key: str) -> GetResult:
        """Return the value plus LWW metadata; raise KeyNotFound if absent."""
        status, text = self._request("GET", c.get_path(key))
        data = c.parse_get(status, text, key=key, url=key)
        return GetResult.from_json(data)

    def get(self, key: str) -> str:
        """Return the value for *key*; raise KeyNotFound if absent."""
        return self.get_full(key).value

    def get_or_none(self, key: str, default: str | None = None) -> str | None:
        """Return the value for *key*, or *default* if it does not exist."""
        try:
            return self.get(key)
        except KeyNotFound:
            return default

    def set(self, key: str, value: str) -> None:
        """Write *value* (a string) under *key*."""
        if not isinstance(value, str):
            raise TypeError("value must be a str; use set_json for other types")
        status, text = self._request(
            "POST", c.SET_PATH, json={"key": key, "value": value}
        )
        c.parse_set(status, text, url=c.SET_PATH)

    def delete(self, key: str) -> bool:
        """Delete *key*; return True if it existed, False otherwise."""
        status, text = self._request("DELETE", c.del_path(key))
        return c.parse_delete(status, text, url=key)

    def exists(self, key: str) -> bool:
        return self.get_or_none(key, _MISSING) is not _MISSING

    # -- JSON convenience ---------------------------------------------------

    def set_json(self, key: str, value: Any, **dumps_kw: Any) -> None:
        """Serialise *value* to JSON and store it as the string value."""
        self.set(key, _json.dumps(value, **dumps_kw))

    def get_json(self, key: str) -> Any:
        """Fetch *key* and JSON-decode it; raise KeyNotFound if absent."""
        return _json.loads(self.get(key))

    # -- dict-style sugar ---------------------------------------------------

    def __getitem__(self, key: str) -> str:
        return self.get(key)

    def __setitem__(self, key: str, value: str) -> None:
        self.set(key, value)

    def __delitem__(self, key: str) -> None:
        if not self.delete(key):
            raise KeyNotFound(key)

    def __contains__(self, key: str) -> bool:
        return self.exists(key)

    # -- inspection ---------------------------------------------------------

    def list_keys(self, page: int = 1, page_size: int = 20) -> KeyPage:
        """List one page of the *responding node's* local keys.

        Note: this reflects a single node's shard, not the whole keyspace.
        """
        status, text = self._request(
            "GET", c.KEYS_PATH, params={"page": page, "pageSize": page_size}
        )
        return KeyPage.from_json(c.parse_json(status, text, url=c.KEYS_PATH))

    def replicas(self, key: str) -> ReplicaInfo:
        """Return replica placement and which nodes currently hold *key*."""
        status, text = self._request("GET", c.replicas_path(key))
        return ReplicaInfo.from_json(c.parse_json(status, text, url=key))

    def health(self) -> dict[str, Any]:
        status, text = self._request("GET", c.HEALTH_PATH)
        return c.parse_json(status, text, url=c.HEALTH_PATH)

    def cluster_state(self) -> dict[str, Any]:
        status, text = self._request("GET", c.CLUSTER_STATE_PATH)
        return c.parse_json(status, text, url=c.CLUSTER_STATE_PATH)

    def ring_state(self) -> dict[str, Any]:
        status, text = self._request("GET", c.CLUSTER_RING_PATH)
        return c.parse_json(status, text, url=c.CLUSTER_RING_PATH)

    def gossip_metrics(self) -> dict[str, Any]:
        status, text = self._request("GET", c.CLUSTER_GOSSIP_PATH)
        return c.parse_json(status, text, url=c.CLUSTER_GOSSIP_PATH)


_MISSING = object()
