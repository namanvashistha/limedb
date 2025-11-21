import asyncio
from client import ClusterClient

async def main():
    client = ClusterClient()
    print("Checking node status...")
    status = await client.get_all_nodes_status()
    for url, data in status.items():
        print(f"URL {url}: {data}")

    print("\nChecking ring state...")
    ring = await client.get_ring_state()
    print(f"Ring: {ring.keys()}")
    if "ranges" in ring:
        print(f"Ranges count: {len(ring['ranges'])}")

if __name__ == "__main__":
    asyncio.run(main())
