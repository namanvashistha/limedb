"""Unit tests for the sync and async clients, using mocked HTTP (respx)."""

from __future__ import annotations

import httpx
import pytest
import respx

from limedb import AsyncClient, Client, ClusterUnavailable, KeyNotFound, RequestError

BASE = "http://node:8484"


# --------------------------------------------------------------------------- sync


@respx.mock
def test_get_set_delete():
    respx.post(f"{BASE}/api/v1/set").mock(return_value=httpx.Response(200, text="OK"))
    respx.get(f"{BASE}/api/v1/get/greeting").mock(
        return_value=httpx.Response(
            200, json={"value": "hello", "timestamp_micros": 42, "nodeUrl": BASE}
        )
    )
    respx.delete(f"{BASE}/api/v1/del/greeting").mock(
        return_value=httpx.Response(200, text="1")
    )

    with Client(BASE) as db:
        db.set("greeting", "hello")
        assert db.get("greeting") == "hello"
        full = db.get_full("greeting")
        assert full.timestamp_micros == 42 and full.node_url == BASE
        assert db.delete("greeting") is True


@respx.mock
def test_get_missing_raises_keynotfound():
    respx.get(f"{BASE}/api/v1/get/nope").mock(
        return_value=httpx.Response(404, text="key not found")
    )
    with Client(BASE) as db:
        with pytest.raises(KeyNotFound):
            db.get("nope")
        assert db.get_or_none("nope") is None
        assert db.get_or_none("nope", "fallback") == "fallback"


@respx.mock
def test_delete_absent_returns_false():
    respx.delete(f"{BASE}/api/v1/del/x").mock(return_value=httpx.Response(200, text="0"))
    with Client(BASE) as db:
        assert db.delete("x") is False


@respx.mock
def test_dict_sugar_and_exists():
    store: dict[str, str] = {}

    def set_handler(request):
        import json

        body = json.loads(request.content)
        store[body["key"]] = body["value"]
        return httpx.Response(200, text="OK")

    def get_handler(request):
        key = request.url.path.rsplit("/", 1)[-1]
        if key in store:
            return httpx.Response(
                200, json={"value": store[key], "timestamp_micros": 1, "nodeUrl": BASE}
            )
        return httpx.Response(404, text="miss")

    respx.post(f"{BASE}/api/v1/set").mock(side_effect=set_handler)
    respx.get(url__regex=rf"{BASE}/api/v1/get/.*").mock(side_effect=get_handler)

    with Client(BASE) as db:
        db["a"] = "1"
        assert db["a"] == "1"
        assert "a" in db
        assert "b" not in db


@respx.mock
def test_json_helpers():
    captured = {}

    def set_handler(request):
        import json

        captured.update(json.loads(request.content))
        return httpx.Response(200, text="OK")

    respx.post(f"{BASE}/api/v1/set").mock(side_effect=set_handler)
    respx.get(f"{BASE}/api/v1/get/cfg").mock(
        return_value=httpx.Response(
            200,
            json={"value": '{"n": 5}', "timestamp_micros": 1, "nodeUrl": BASE},
        )
    )
    with Client(BASE) as db:
        db.set_json("cfg", {"n": 5})
        assert captured["value"] == '{"n": 5}'
        assert db.get_json("cfg") == {"n": 5}


@respx.mock
def test_key_encoding():
    route = respx.get(f"{BASE}/api/v1/get/a%2Fb%20c").mock(
        return_value=httpx.Response(
            200, json={"value": "v", "timestamp_micros": 1, "nodeUrl": BASE}
        )
    )
    with Client(BASE) as db:
        assert db.get("a/b c") == "v"
    assert route.called


@respx.mock
def test_failover_to_second_seed():
    respx.get("http://dead:8484/api/v1/get/k").mock(
        side_effect=httpx.ConnectError("down")
    )
    respx.get("http://live:8484/api/v1/get/k").mock(
        return_value=httpx.Response(
            200, json={"value": "v", "timestamp_micros": 1, "nodeUrl": "live"}
        )
    )
    with Client(["http://dead:8484", "http://live:8484"]) as db:
        assert db.get("k") == "v"
        # live seed is now preferred; a second call should hit it first
        assert db.get("k") == "v"


@respx.mock
def test_all_seeds_down_raises():
    respx.get(url__regex=r".*").mock(side_effect=httpx.ConnectError("down"))
    with Client(["http://a:8484", "http://b:8484"]) as db:
        with pytest.raises(ClusterUnavailable):
            db.get("k")


@respx.mock
def test_server_error_raises_requesterror():
    respx.post(f"{BASE}/api/v1/set").mock(return_value=httpx.Response(500, text="boom"))
    with Client(BASE) as db:
        with pytest.raises(RequestError):
            db.set("k", "v")


def test_set_rejects_non_str():
    with Client(BASE) as db:
        with pytest.raises(TypeError):
            db.set("k", 123)  # type: ignore[arg-type]


def test_empty_seeds_rejected():
    with pytest.raises(ValueError):
        Client([])


# --------------------------------------------------------------------------- async


@respx.mock
async def test_async_get_set():
    respx.post(f"{BASE}/api/v1/set").mock(return_value=httpx.Response(200, text="OK"))
    respx.get(f"{BASE}/api/v1/get/k").mock(
        return_value=httpx.Response(
            200, json={"value": "v", "timestamp_micros": 7, "nodeUrl": BASE}
        )
    )
    async with AsyncClient(BASE) as db:
        await db.set("k", "v")
        assert await db.get("k") == "v"
        assert await db.get_or_none("k") == "v"


@respx.mock
async def test_async_missing():
    respx.get(f"{BASE}/api/v1/get/nope").mock(return_value=httpx.Response(404, text="miss"))
    async with AsyncClient(BASE) as db:
        with pytest.raises(KeyNotFound):
            await db.get("nope")
        assert await db.get_or_none("nope", "d") == "d"
