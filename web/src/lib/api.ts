import { GossipMetrics, KeyValueResponse, NodeStatus, RingState } from "./types";

const BASE_URL = "/api/proxy";

class ClusterClient {
  private discoveredHosts: Set<string> = new Set();
  private seedUrl: string;

  constructor(seedUrl: string = "http://192.168.1.124:8484") {
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

  // Derive cluster status from gossip metrics and ring state
  async getClusterStatus(): Promise<{ 
    gossip: GossipMetrics; 
    nodes: Record<string, NodeStatus>; 
    selfNode: string | null;
  }> {
    const [gossip, ring] = await Promise.all([
      this.getGossipMetrics(),
      this.getRingState(),
    ]);
    
    // Get self node from ring state
    const selfNode = ring.currentNode as string || null;
    
    // Build node status map from gossip peer_details
    const nodes: Record<string, NodeStatus> = {};
    
    // Add self node first if available
    if (selfNode && gossip.peer_details) {
      const selfPeer = gossip.peer_details.find(p => p.url === selfNode);
      if (selfPeer) {
        nodes[selfNode] = {
          nodeUrl: selfNode,
          status: selfPeer.status,
          peers: gossip.peer_details.filter(p => p.url !== selfNode).map(p => p.url),
          totalNodes: gossip.total_peers,
          isSelf: true,
        };
      }
    }
    
    // Add peer nodes
    if (gossip.peer_details && gossip.peer_details.length > 0) {
      gossip.peer_details.forEach((peer) => {
        if (peer.url !== selfNode && !nodes[peer.url]) {
          nodes[peer.url] = {
            nodeUrl: peer.url,
            status: peer.status,
            peers: gossip.peer_details.filter(p => p.url !== peer.url).map(p => p.url),
            totalNodes: gossip.total_peers,
            isSelf: false,
          };
        }
      });
    }
    
    return { gossip, nodes, selfNode };
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

  async listKeys(page: number = 1, pageSize: number = 20): Promise<{
    keys: Array<{ key: string; value: string; size: number }>;
    total: number;
    page: number;
    pageSize: number;
    totalPages: number;
  }> {
    const res = await fetch(`${BASE_URL}/keys?page=${page}&pageSize=${pageSize}`);
    if (!res.ok) throw new Error("Failed to fetch keys");
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
