"""Exception types raised by the LimeDB SDK."""

from __future__ import annotations


class LimeDBError(Exception):
    """Base class for all SDK errors."""


class KeyNotFound(LimeDBError, KeyError):
    """Raised by ``get`` when a key does not exist (HTTP 404).

    Subclasses ``KeyError`` so ``client[key]`` behaves like a dict lookup.
    """

    def __init__(self, key: str):
        self.key = key
        super().__init__(f"key not found: {key!r}")


class RequestError(LimeDBError):
    """The server returned a non-2xx status other than 404."""

    def __init__(self, status_code: int, body: str, *, url: str):
        self.status_code = status_code
        self.body = body
        self.url = url
        super().__init__(f"{status_code} from {url}: {body.strip()[:200]}")


class ClusterUnavailable(LimeDBError):
    """Every seed node failed to answer (connection/timeout errors)."""

    def __init__(self, tried: list[str], last_error: Exception | None):
        self.tried = tried
        self.last_error = last_error
        super().__init__(
            f"no LimeDB node reachable (tried {len(tried)}: {', '.join(tried)}): "
            f"{last_error!r}"
        )
