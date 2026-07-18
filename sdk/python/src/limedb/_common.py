"""Shared helpers for the sync and async clients.

Both clients speak the same HTTP protocol; only the transport (blocking vs
awaitable httpx) differs. Everything that is transport-independent — seed URL
normalisation, path building, status-code interpretation — lives here so the
two clients stay in lockstep.
"""

from __future__ import annotations

import json
from typing import Any
from urllib.parse import quote

from .errors import KeyNotFound, RequestError

DEFAULT_TIMEOUT = 5.0
API = "/api/v1"


def normalize_seeds(urls: str | list[str] | tuple[str, ...]) -> list[str]:
    """Accept a single URL or an iterable of them; strip trailing slashes."""
    if isinstance(urls, str):
        urls = [urls]
    seeds = [u.rstrip("/") for u in urls if u and u.strip()]
    if not seeds:
        raise ValueError("at least one seed URL is required")
    return seeds


def encode_key(key: str) -> str:
    """URL-encode a key for use in a path segment.

    ``safe=""`` so slashes in keys don't create extra path segments.
    """
    if key == "":
        raise ValueError("key must not be empty")
    return quote(key, safe="")


# --- path builders (relative to a node base URL) ---------------------------

def get_path(key: str) -> str:
    return f"{API}/get/{encode_key(key)}"


def del_path(key: str) -> str:
    return f"{API}/del/{encode_key(key)}"


def replicas_path(key: str) -> str:
    return f"{API}/replicas/{encode_key(key)}"


SET_PATH = f"{API}/set"
KEYS_PATH = f"{API}/keys"
HEALTH_PATH = f"{API}/health"
CLUSTER_STATE_PATH = f"{API}/cluster/state"
CLUSTER_RING_PATH = f"{API}/cluster/ring"
CLUSTER_GOSSIP_PATH = f"{API}/cluster/gossip"


# --- response interpretation -----------------------------------------------

def parse_json(status: int, text: str, *, url: str) -> dict[str, Any]:
    """Validate a JSON response, raising RequestError on non-2xx."""
    if status == 404:
        # only reachable for endpoints where 404 is a real error, not a miss
        raise RequestError(status, text, url=url)
    if not 200 <= status < 300:
        raise RequestError(status, text, url=url)
    try:
        return json.loads(text)
    except json.JSONDecodeError as exc:
        raise RequestError(status, f"invalid JSON: {exc}", url=url) from exc


def parse_get(status: int, text: str, *, key: str, url: str) -> dict[str, Any]:
    """GET returns 404 when the key is absent — translate to KeyNotFound."""
    if status == 404:
        raise KeyNotFound(key)
    return parse_json(status, text, url=url)


def parse_delete(status: int, text: str, *, url: str) -> bool:
    """DELETE returns the body ``"1"`` (deleted) or ``"0"`` (was absent)."""
    if not 200 <= status < 300:
        raise RequestError(status, text, url=url)
    return text.strip() == "1"


def parse_set(status: int, text: str, *, url: str) -> None:
    if not 200 <= status < 300:
        raise RequestError(status, text, url=url)
