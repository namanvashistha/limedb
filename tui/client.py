import aiohttp
import asyncio
from typing import List, Dict, Any

class ClusterClient:
    def __init__(self, base_ports: List[int] = [7001, 7002, 7003, 7004, 7005]):
        self.base_ports = base_ports
        self.base_urls = [f"http://localhost:{port}/api/v1" for port in base_ports]

    async def get_node_status(self, port: int) -> Dict[str, Any]:
        url = f"http://localhost:{port}/api/v1/cluster/state"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=2) as response:
                    if response.status == 200:
                        return await response.json()
                    return {"error": f"Status {response.status}"}
        except Exception as e:
            return {"error": str(e)}

    async def get_all_nodes_status(self) -> Dict[int, Dict[str, Any]]:
        tasks = [self.get_node_status(port) for port in self.base_ports]
        results = await asyncio.gather(*tasks)
        return {port: result for port, result in zip(self.base_ports, results)}

    async def get_ring_state(self, port: int = 7001) -> Dict[str, Any]:
        url = f"http://localhost:{port}/api/v1/cluster/ring"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=2) as response:
                    if response.status == 200:
                        return await response.json()
                    return {"error": f"Status {response.status}"}
        except Exception as e:
            return {"error": repr(e)}

    async def set_key(self, key: str, value: str, port: int = 7001) -> Dict[str, Any]:
        url = f"http://localhost:{port}/api/v1/set"
        payload = {"key": key, "value": value}
        try:
            async with aiohttp.ClientSession() as session:
                async with session.post(url, json=payload, timeout=2) as response:
                    text = await response.text()
                    return {"status": response.status, "body": text}
        except Exception as e:
            return {"error": repr(e)}

    async def get_key(self, key: str, port: int = 7001) -> Dict[str, Any]:
        url = f"http://localhost:{port}/api/v1/get/{key}"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=2) as response:
                    text = await response.text()
                    return {"status": response.status, "body": text}
        except Exception as e:
            return {"error": repr(e)}

    async def delete_key(self, key: str, port: int = 7001) -> Dict[str, Any]:
        url = f"http://localhost:{port}/api/v1/del/{key}"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.delete(url, timeout=2) as response:
                    text = await response.text()
                    return {"status": response.status, "body": text}
        except Exception as e:
            return {"error": repr(e)}
