"""Asynchronous LimeDB client (mirror of :class:`limedb.client.Client`)."""

from __future__ import annotations

import json as _json
from typing import Any, Iterable

import httpx

from . import _common as c
from .errors import ClusterUnavailable, KeyNotFound
from .models import GetResult, KeyPage, ReplicaInfo

_MISSING = object()


class AsyncClient:
    """An asyncio client for a LimeDB cluster.

    Behaviour matches :class:`limedb.Client` but every network method is a
    coroutine. Use as an async context manager::

        async with AsyncClient("http://localhost:8484") as db:
            await db.set("greeting", "hello")
            print(await db.get("greeting"))
    """

    def __init__(
        self,
        urls: str | Iterable[str],
        *,
        timeout: float = c.DEFAULT_TIMEOUT,
        client: httpx.AsyncClient | None = None,
    ):
        self._seeds = c.normalize_seeds(list(urls) if not isinstance(urls, str) else urls)
        self._preferred = 0
        self._owns_client = client is None
        self._http = client or httpx.AsyncClient(timeout=timeout)

    # -- lifecycle ----------------------------------------------------------

    async def close(self) -> None:
        if self._owns_client:
            await self._http.aclose()

    async def __aenter__(self) -> "AsyncClient":
        return self

    async def __aexit__(self, *exc: object) -> None:
        await self.close()

    # -- request plumbing ---------------------------------------------------

    def _ordered_seeds(self) -> list[str]:
        p = self._preferred
        return self._seeds[p:] + self._seeds[:p]

    async def _request(self, method: str, path: str, **kw: Any) -> tuple[int, str]:
        tried: list[str] = []
        last_err: Exception | None = None
        for offset, base in enumerate(self._ordered_seeds()):
            url = f"{base}{path}"
            tried.append(base)
            try:
                resp = await self._http.request(method, url, **kw)
            except httpx.TransportError as exc:
                last_err = exc
                continue
            self._preferred = (self._preferred + offset) % len(self._seeds)
            return resp.status_code, resp.text
        raise ClusterUnavailable(tried, last_err)

    # -- key/value ----------------------------------------------------------

    async def get_full(self, key: str) -> GetResult:
        status, text = await self._request("GET", c.get_path(key))
        data = c.parse_get(status, text, key=key, url=key)
        return GetResult.from_json(data)

    async def get(self, key: str) -> str:
        return (await self.get_full(key)).value

    async def get_or_none(self, key: str, default: str | None = None) -> str | None:
        try:
            return await self.get(key)
        except KeyNotFound:
            return default

    async def set(self, key: str, value: str) -> None:
        if not isinstance(value, str):
            raise TypeError("value must be a str; use set_json for other types")
        status, text = await self._request(
            "POST", c.SET_PATH, json={"key": key, "value": value}
        )
        c.parse_set(status, text, url=c.SET_PATH)

    async def delete(self, key: str) -> bool:
        status, text = await self._request("DELETE", c.del_path(key))
        return c.parse_delete(status, text, url=key)

    async def exists(self, key: str) -> bool:
        return await self.get_or_none(key, _MISSING) is not _MISSING  # type: ignore[arg-type]

    # -- JSON convenience ---------------------------------------------------

    async def set_json(self, key: str, value: Any, **dumps_kw: Any) -> None:
        await self.set(key, _json.dumps(value, **dumps_kw))

    async def get_json(self, key: str) -> Any:
        return _json.loads(await self.get(key))

    # -- inspection ---------------------------------------------------------

    async def list_keys(self, page: int = 1, page_size: int = 20) -> KeyPage:
        status, text = await self._request(
            "GET", c.KEYS_PATH, params={"page": page, "pageSize": page_size}
        )
        return KeyPage.from_json(c.parse_json(status, text, url=c.KEYS_PATH))

    async def replicas(self, key: str) -> ReplicaInfo:
        status, text = await self._request("GET", c.replicas_path(key))
        return ReplicaInfo.from_json(c.parse_json(status, text, url=key))

    async def health(self) -> dict[str, Any]:
        status, text = await self._request("GET", c.HEALTH_PATH)
        return c.parse_json(status, text, url=c.HEALTH_PATH)

    async def cluster_state(self) -> dict[str, Any]:
        status, text = await self._request("GET", c.CLUSTER_STATE_PATH)
        return c.parse_json(status, text, url=c.CLUSTER_STATE_PATH)

    async def ring_state(self) -> dict[str, Any]:
        status, text = await self._request("GET", c.CLUSTER_RING_PATH)
        return c.parse_json(status, text, url=c.CLUSTER_RING_PATH)

    async def gossip_metrics(self) -> dict[str, Any]:
        status, text = await self._request("GET", c.CLUSTER_GOSSIP_PATH)
        return c.parse_json(status, text, url=c.CLUSTER_GOSSIP_PATH)
