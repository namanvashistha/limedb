import asyncio
from client import ClusterClient

async def main():
    client = ClusterClient()
    print("Checking node status...")
    status = await client.get_all_nodes_status()
    for port, data in status.items():
        print(f"Port {port}: {data}")

    print("\nChecking ring state (from port 7001)...")
    ring = await client.get_ring_state(7001)
    print(f"Ring: {ring.keys()}")
    if "ranges" in ring:
        print(f"Ranges count: {len(ring['ranges'])}")

if __name__ == "__main__":
    asyncio.run(main())
