export interface NodeStatus {
  nodeUrl: string;
  status: "active" | "dead" | "stale" | "unknown";
  peers: string[];
  totalNodes: number;
  isSelf?: boolean;
}

export interface GossipMetrics {
  node_url: string;
  cluster_health: "healthy" | "degraded" | "critical" | "unknown";
  node_heartbeat: number;
  total_peers: number;
  active_peers: number;
  dead_peers: number;
  stale_peers: number;
  convergence_rate: number;
  average_lag: number;
  max_lag: number;
  peer_details: PeerDetail[];
  status?: string;
  timestamp?: number;
}

export interface PeerDetail {
  url: string;
  heartbeat: number;
  lag: number;
  status: "active" | "stale" | "dead" | "unknown";
}

export interface RingRange {
  start: number;
  end: number;
  node: string;
  size: number;
}

export interface RingState {
  ranges: Record<string, RingRange[]>;
  version?: number;
  currentNode?: string;
  allNodes?: string[];
  virtualNodesPerNode?: number;
}

export interface KeyValueResponse {
  status: number;
  body: string | object;
  error?: string;
}
