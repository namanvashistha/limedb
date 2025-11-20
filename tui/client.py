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

    async def get_metrics(self, port: int = 7001) -> Dict[str, Any]:
        """Fetches JVM metrics from Actuator."""
        metrics = {}
        base_url = f"http://localhost:{port}/actuator/metrics"
        
        keys = {
            "jvm.memory.used": "memory",
            "system.cpu.usage": "cpu",
            "process.uptime": "uptime"
        }
        
        async with aiohttp.ClientSession() as session:
            for metric_name, alias in keys.items():
                try:
                    async with session.get(f"{base_url}/{metric_name}", timeout=1) as response:
                        if response.status == 200:
                            data = await response.json()
                            measurements = data.get("measurements", [])
                            if measurements:
                                metrics[alias] = measurements[0].get("value", 0)
                except Exception:
                    metrics[alias] = -1
        return metrics

    async def measure_latency(self, port: int = 7001) -> float:
        """Measures latency to the node in milliseconds."""
        url = f"http://localhost:{port}/api/v1/cluster/state"
        start = asyncio.get_event_loop().time()
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=1) as response:
                    await response.read()
                    end = asyncio.get_event_loop().time()
                    return (end - start) * 1000
        except Exception:
            return 9999.0
