import { GossipMetrics, KeyValueResponse, NodeStatus, RingState } from "./types";

const BASE_URL = "/api/proxy";

class ClusterClient {
  private discoveredHosts: Set<string> = new Set();
  private seedUrl: string;

  constructor(seedUrl: string = "http://192.168.0.124:8484") {
    this.seedUrl = seedUrl;
    this.discoveredHosts.add(seedUrl);
  }

  // Discover all nodes in cluster through gossip
  async discoverCluster(): Promise<string[]> {
    try {
      const gossip = await this.getGossipMetrics();
      
      // Add peer URLs from gossip
      if (gossip.peer_details && gossip.peer_details.length > 0) {
        gossip.peer_details.forEach((peer) => {
          if (peer.url && !this.discoveredHosts.has(peer.url)) {
            this.discoveredHosts.add(peer.url);
          }
        });
      }

      return Array.from(this.discoveredHosts);
    } catch (error) {
      console.error("Failed to discover cluster:", error);
      return Array.from(this.discoveredHosts);
    }
  }

  async getClusterStatus(): Promise<NodeStatus> {
    const res = await fetch(`${BASE_URL}/cluster/state`);
    if (!res.ok) throw new Error("Failed to fetch cluster status");
    return res.json();
  }

  // Get status from all discovered nodes
  async getAllNodesStatus(): Promise<Record<string, NodeStatus | { error: string }>> {
    // First discover all nodes
    await this.discoverCluster();
    
    const hosts = Array.from(this.discoveredHosts);
    const statusMap: Record<string, NodeStatus | { error: string }> = {};

    // Query all nodes in parallel
    const results = await Promise.allSettled(
      hosts.map(async (host) => {
        try {
          // For now, we only query the seed through proxy
          // In a full implementation, we'd proxy requests to all hosts
          const res = await fetch(`${BASE_URL}/cluster/state`);
          if (!res.ok) throw new Error("Failed to fetch");
          const data = await res.json();
          return { host, data };
        } catch (error) {
          return { host, data: { error: error instanceof Error ? error.message : "Unknown error" } };
        }
      })
    );

    results.forEach((result) => {
      if (result.status === "fulfilled") {
        statusMap[result.value.host] = result.value.data;
      }
    });

    return statusMap;
  }

  async getRingState(): Promise<RingState> {
    const res = await fetch(`${BASE_URL}/cluster/ring`);
    if (!res.ok) throw new Error("Failed to fetch ring state");
    return res.json();
  }

  async getGossipMetrics(): Promise<GossipMetrics> {
    const res = await fetch(`${BASE_URL}/cluster/gossip`);
    if (!res.ok) throw new Error("Failed to fetch gossip metrics");
    return res.json();
  }

  async getKey(key: string): Promise<KeyValueResponse> {
    const res = await fetch(`${BASE_URL}/get/${key}`);
    const body = await res.text();
    return { status: res.status, body };
  }

  async setKey(key: string, value: string): Promise<KeyValueResponse> {
    const res = await fetch(`${BASE_URL}/set`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value }),
    });
    const body = await res.text();
    return { status: res.status, body };
  }

  async deleteKey(key: string): Promise<KeyValueResponse> {
    const res = await fetch(`${BASE_URL}/del/${key}`, {
      method: "DELETE",
    });
    const body = await res.text();
    return { status: res.status, body };
  }

  getDiscoveredHosts(): string[] {
    return Array.from(this.discoveredHosts);
  }
}

// Export singleton instance
export const api = new ClusterClient();
