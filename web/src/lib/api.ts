import { GossipMetrics, HealthResponse, KeyValueResponse, NodeStatus, RingState } from "./types";

const BASE_URL = "/api/proxy";

class ClusterClient {
  private discoveredHosts: Set<string> = new Set();
  private seedUrl: string;

  constructor(seedUrl: string = "http://node1:8484") {
    this.seedUrl = seedUrl;
    this.discoveredHosts.add(seedUrl);
  }

  setSeedUrl(url: string) {
    this.seedUrl = url;
    this.discoveredHosts.add(url);
  }

  getSeedUrl(): string {
    return this.seedUrl;
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

  // Derive cluster status from gossip metrics and ring state
  // NO SELF NODE CONCEPT - all nodes are peers
  async getClusterStatus(): Promise<{ 
    gossip: GossipMetrics; 
    nodes: Record<string, NodeStatus>;
  }> {
    const [gossip, ring] = await Promise.all([
      this.getGossipMetrics(),
      this.getRingState(),
    ]);
    
    // Build node status map from gossip peer_details - ALL NODES EQUAL
    const nodes: Record<string, NodeStatus> = {};
    
    // Add all peer nodes from gossip
    if (gossip.peer_details && gossip.peer_details.length > 0) {
      gossip.peer_details.forEach((peer) => {
        if (!nodes[peer.url]) {
          nodes[peer.url] = {
            nodeUrl: peer.url,
            status: peer.status,
            peers: gossip.peer_details.filter(p => p.url !== peer.url).map(p => p.url),
            totalNodes: gossip.total_peers,
            isSelf: false, // All nodes are peers - no special treatment
          };
        }
      });
    }
    
    return { gossip, nodes };
  }

  async getRingState(): Promise<RingState> {
    const query = `?node=${encodeURIComponent(this.seedUrl)}`;
    const res = await fetch(`${BASE_URL}/cluster/ring${query}`);
    if (!res.ok) throw new Error("Failed to fetch ring state");
    return res.json();
  }

  async getGossipMetrics(nodeUrl?: string): Promise<GossipMetrics> {
    // If nodeUrl is provided, use it. Otherwise, use the configured seedUrl.
    // We pass it as a query param to the proxy.
    const target = nodeUrl || this.seedUrl;
    const query = `?node=${encodeURIComponent(target)}`;
    const response = await fetch(`${BASE_URL}/cluster/gossip${query}`);
    if (!response.ok) {
      throw new Error("Failed to fetch gossip metrics");
    }
    return response.json();
  }

  async getHealth(nodeUrl?: string): Promise<HealthResponse> {
    const target = nodeUrl || this.seedUrl;
    const query = `?node=${encodeURIComponent(target)}`;
    const response = await fetch(`${BASE_URL}/health${query}`);
    if (!response.ok) {
      throw new Error("Failed to fetch node health");
    }
    return response.json();
  }

  async listKeys(page: number = 1, pageSize: number = 20): Promise<{
    keys: Array<{ key: string; value: string; size: number }>;
    total: number;
    page: number;
    pageSize: number;
    totalPages: number;
  }> {
    const query = `&node=${encodeURIComponent(this.seedUrl)}`;
    const res = await fetch(`${BASE_URL}/keys?page=${page}&pageSize=${pageSize}${query}`);
    if (!res.ok) throw new Error("Failed to fetch keys");
    return res.json();
  }

  // Fetch keys from a specific node URL
  async listKeysFromNode(nodeUrl: string, page: number = 1, pageSize: number = 200): Promise<{
    keys: Array<{ key: string; value: string; size: number; nodeUrl: string }>;
    total: number;
  }> {
    const query = `&node=${encodeURIComponent(nodeUrl)}`;
    const res = await fetch(`${BASE_URL}/keys?page=${page}&pageSize=${pageSize}${query}`);
    if (!res.ok) throw new Error(`Failed to fetch keys from ${nodeUrl}`);
    const data = await res.json();
    return {
      keys: (data.keys || []).map((k: { key: string; value: string; size: number }) => ({ ...k, nodeUrl })),
      total: data.total || 0,
    };
  }

  // Fan out to all discovered nodes, merge and deduplicate by key
  async listAllKeys(): Promise<Array<{ key: string; value: string; size: number; nodeUrl: string }>> {
    const nodes = await this.discoverCluster();
    const results = await Promise.allSettled(
      nodes.map((nodeUrl) => this.listKeysFromNode(nodeUrl))
    );

    const seen = new Map<string, { key: string; value: string; size: number; nodeUrl: string }>();
    for (const result of results) {
      if (result.status === "fulfilled") {
        for (const item of result.value.keys) {
          if (!seen.has(item.key)) {
            seen.set(item.key, item);
          }
        }
      }
    }
    return Array.from(seen.values()).sort((a, b) => a.key.localeCompare(b.key));
  }

  async getKey(key: string): Promise<KeyValueResponse> {
    const query = `?node=${encodeURIComponent(this.seedUrl)}`;
    const res = await fetch(`${BASE_URL}/get/${key}${query}`);
    const body = await res.text();
    return { status: res.status, body };
  }

  async setKey(key: string, value: string, nodeUrl?: string): Promise<KeyValueResponse> {
    const target = nodeUrl || this.seedUrl;
    const query = `?node=${encodeURIComponent(target)}`;
    const res = await fetch(`${BASE_URL}/set${query}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value }),
    });
    const body = await res.text();
    return { status: res.status, body };
  }

  async deleteKey(key: string): Promise<KeyValueResponse> {
    const query = `?node=${encodeURIComponent(this.seedUrl)}`;
    const res = await fetch(`${BASE_URL}/del/${key}${query}`, {
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
