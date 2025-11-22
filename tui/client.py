import aiohttp
import asyncio
import random
from typing import List, Dict, Any, Optional


class ClusterClient:
    def __init__(self, base_urls: List[str]):
        if not base_urls:
            raise ValueError("base_urls cannot be empty")
        self.base_urls = base_urls
        self.base_api_urls = [f"{url}/api/v1" for url in base_urls]
        self.discovered_hosts = set(base_urls)  # Track all discovered hosts

    def get_random_url(self) -> str:
        """Returns a random URL from the available base URLs."""
        return random.choice(self.base_urls)

    async def get_node_status(self, base_url: str) -> Dict[str, Any]:
        url = f"{base_url}/api/v1/cluster/state"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=2) as response:
                    if response.status == 200:
                        return await response.json()
                    return {"error": f"Status {response.status}"}
        except Exception as e:
            return {"error": str(e)}

    async def get_all_nodes_status(self) -> Dict[str, Dict[str, Any]]:
        tasks = [self.get_node_status(base_url) for base_url in self.base_urls]
        results = await asyncio.gather(*tasks)
        return {base_url: result for base_url, result in zip(self.base_urls, results)}

    async def get_ring_state(self, base_url: Optional[str] = None) -> Dict[str, Any]:
        if base_url is None:
            base_url = self.get_random_url()
        url = f"{base_url}/api/v1/cluster/ring"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=2) as response:
                    if response.status == 200:
                        return await response.json()
                    return {"error": f"Status {response.status}"}
        except Exception as e:
            return {"error": repr(e)}

    async def get_gossip_metrics(self, base_url: Optional[str] = None) -> Dict[str, Any]:
        if base_url is None:
            base_url = self.get_random_url()
        url = f"{base_url}/api/v1/cluster/gossip"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=2) as response:
                    if response.status == 200:
                        return await response.json()
                    return {"error": f"Status {response.status}"}
        except Exception as e:
            return {"error": repr(e)}

    async def set_key(
        self, key: str, value: str, base_url: Optional[str] = None
    ) -> Dict[str, Any]:
        if base_url is None:
            base_url = self.get_random_url()
        url = f"{base_url}/api/v1/set"
        payload = {"key": key, "value": value}
        try:
            async with aiohttp.ClientSession() as session:
                async with session.post(url, json=payload, timeout=2) as response:
                    text = await response.text()
                    return {"status": response.status, "body": text}
        except Exception as e:
            return {"error": repr(e)}

    async def get_key(self, key: str, base_url: Optional[str] = None) -> Dict[str, Any]:
        if base_url is None:
            base_url = self.get_random_url()
        url = f"{base_url}/api/v1/get/{key}"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=2) as response:
                    text = await response.text()
                    return {"status": response.status, "body": text}
        except Exception as e:
            return {"error": repr(e)}

    async def delete_key(
        self, key: str, base_url: Optional[str] = None
    ) -> Dict[str, Any]:
        if base_url is None:
            base_url = self.get_random_url()
        url = f"{base_url}/api/v1/del/{key}"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.delete(url, timeout=2) as response:
                    text = await response.text()
                    return {"status": response.status, "body": text}
        except Exception as e:
            return {"error": repr(e)}

    def update_discovered_hosts(self, gossip_data: Dict[str, Any], current_node_url: str) -> None:
        """Update the list of discovered hosts based on gossip data."""
        # Add current node
        self.discovered_hosts.add(current_node_url)
        
        # Add peers from gossip data
        peer_details = gossip_data.get("peer_details", [])
        for peer in peer_details:
            peer_url = peer.get("url")
            if peer_url and peer_url not in self.discovered_hosts:
                self.discovered_hosts.add(peer_url)
                
        # Update base_urls and base_api_urls with newly discovered hosts
        new_hosts = sorted(list(self.discovered_hosts))
        if set(new_hosts) != set(self.base_urls):
            self.base_urls = new_hosts
            self.base_api_urls = [f"{url}/api/v1" for url in new_hosts]
            
    def get_all_discovered_hosts(self) -> List[str]:
        """Get all discovered hosts."""
        return sorted(list(self.discovered_hosts))

    async def get_metrics(self, base_url: Optional[str] = None) -> Dict[str, Any]:
        """Fetches basic metrics from the node."""
        if base_url is None:
            base_url = self.get_random_url()
        metrics = {}

        # For now, return dummy metrics since Go version doesn't have actuator
        # TODO: Implement proper metrics endpoint in Go version
        metrics = {"memory": 0, "cpu": 0, "uptime": 0}
        return metrics

    async def measure_latency(self, base_url: Optional[str] = None) -> float:
        """Measures latency to the node in milliseconds."""
        if base_url is None:
            base_url = self.get_random_url()
        url = f"{base_url}/api/v1/cluster/state"
        start = asyncio.get_event_loop().time()
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, timeout=1) as response:
                    await response.read()
                    end = asyncio.get_event_loop().time()
                    return (end - start) * 1000
        except Exception:
            return 9999.0
