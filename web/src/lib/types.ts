export interface NodeStatus {
  nodeUrl: string;
  status: "active" | "dead" | "stale" | "unknown";
  peers: string[];
  totalNodes: number;
  isSelf?: boolean;
}

export interface GossipMetrics {
  node_url: string;
  generation?: number;
  cluster_health: "healthy" | "degraded" | "critical" | "unknown";
  node_heartbeat: number;
  total_peers: number;
  active_peers: number;
  dead_peers: number;
  is_seed?: boolean;
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
  generation?: number;
  heartbeat: number;
  lag: number;
  status: "active" | "stale" | "dead" | "unknown";
}

export interface ReplicaNode {
  node_url: string;
  is_primary: boolean;
  is_local: boolean;
  has_value: boolean;
}

export interface ReplicaInfo {
  key: string;
  replication_factor: number;
  quorum: number;
  replicas: ReplicaNode[];
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

export interface StorageStats {
  type: string;
  memtable_size_b?: number;
  memtable_keys?: number;
  flush_threshold_b?: number;
  sstable_count?: number;
  compaction_count?: number;
  is_compacting?: boolean;
  last_compaction_duration_ms?: number;
  total_disk_usage_b?: number;
  bloom_false_positive_rate?: number;
  bloom_false_positives_total?: number;
  bloom_true_positives_total?: number;
  approx_total_keys?: number;
  keys?: number; // for memory backend
}

export interface HealthResponse {
  status: string;
  service: string;
  version: string;
  nodeUrl: string;
  timestamp: string;
  uptime_seconds?: number;
  memory_allocated_mb?: number;
  memory_sys_mb?: number;
  gc_pause_ms?: number;
  goroutines_count?: number;
  requests_per_second?: number;
  gets_per_second?: number;
  sets_per_second?: number;
  dels_per_second?: number;
  average_latency_ms?: number;
  cpu_percent?: number; // CPU usage percentage (0-100)
  error_rate?: number;
  total_requests?: number;
  storage?: StorageStats;
}

export interface KeyValueResponse {
  status: number;
  body: string | object;
  error?: string;
}

export interface MembershipNode {
  node_url: string;
  state: "DISCOVERED" | "BOOTSTRAPPING" | "ACTIVE" | "LEAVING" | "LEFT";
  is_local: boolean;
  is_active_for_routing?: boolean;
  generation?: number;
  version?: number;
  last_seen_unix?: number;
  liveness?: "unknown" | "alive" | "stale" | "dead";
  activation_epoch?: number;
  transition_requested?: boolean;
}

export interface MembershipState {
  current_node_url: string;
  membership_epoch?: number;
  observed_nodes: MembershipNode[];
  active_nodes: MembershipNode[];
  desired_nodes?: MembershipNode[];
}

export interface PlacementMember {
  node_url: string;
  role: string;
}

export interface TokenAssignment {
  token: number;
  node_url: string;
}

export interface PlacementState {
  active?: PlacementSnapshot;
  pending?: PlacementSnapshot;
}

export interface PlacementSnapshot {
  epoch: number;
  status: "PENDING" | "ACTIVE";
  virtual_nodes: number;
  replication_factor: number;
  members: PlacementMember[];
  tokens: TokenAssignment[];
  created_at_unix: number;
}

export interface BootstrapRange {
  start_token: number;
  end_token: number;
  token: number;
  from_nodes: string[];
  to_node: string;
  status: "PENDING" | "COPYING" | "VERIFIED" | "FAILED";
  keys_copied: number;
}

export interface BootstrapPlan {
  plan_id: string;
  target_node_url: string;
  placement_epoch: number;
  source_epoch: number;
  status: "PLANNED" | "RUNNING" | "COMPLETED" | "FAILED";
  ranges: BootstrapRange[];
  started_at_unix: number;
  completed_at_unix?: number;
}
