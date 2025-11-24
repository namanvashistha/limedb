import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { RingDistributionChart } from "@/components/metrics/RingDistributionChart";
import { api } from "@/lib/api";
import { RingState, GossipMetrics } from "@/lib/types";
import { Activity, Server, Database, AlertCircle } from "lucide-react";

export function ClusterOverview() {
  const [ring, setRing] = useState<RingState | null>(null);
  const [gossip, setGossip] = useState<GossipMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [gossipData, ringData] = await Promise.all([
          api.getGossipMetrics(),
          api.getRingState(),
        ]);
        setGossip(gossipData);
        setRing(ringData);
        setError(null);
      } catch (err) {
        // console.warn("Failed to fetch cluster data", err);
        setError("Failed to connect to cluster");
      } finally {
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 2000);
    return () => clearInterval(interval);
  }, []);

  if (loading && !ring) {
    return <div className="flex items-center justify-center h-full">Loading cluster state...</div>;
  }

  if (error && !ring) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-destructive gap-4">
        <AlertCircle className="h-12 w-12" />
        <div className="text-lg font-medium">{error}</div>
      </div>
    );
  }

  // Calculate total nodes: seed node + peers
  // ringState?.allNodes?.length gives us all nodes if available
  // Otherwise, we use metrics.total_peers + 1 (for seed node)
  const totalNodes = ring?.allNodes?.length || (gossip ? gossip.total_peers + 1 : 0);
  const healthyNodes = gossip ? gossip.active_peers + 1 : 0; // +1 for seed node (always active)
  const deadNodes = gossip?.dead_peers || 0;

  return (
    <div className="space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Nodes</CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalNodes}</div>
            <p className="text-xs text-muted-foreground">
              {healthyNodes} Healthy, {deadNodes} Dead
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Cluster Health</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold capitalize">{gossip?.cluster_health || "Unknown"}</div>
            <p className="text-xs text-muted-foreground">
              Convergence: {(gossip?.convergence_rate || 0).toFixed(1)}%
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Token Distribution</CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{ring?.ranges ? Object.values(ring.ranges).flat().length : 0}</div>
            <p className="text-xs text-muted-foreground">
              Active Ranges
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Ring Visualization */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 h-[500px]">
        <Card className="lg:col-span-2 h-full flex flex-col">
          <CardHeader>
            <CardTitle>Token Ring Distribution</CardTitle>
          </CardHeader>
          <CardContent className="flex-1 min-h-0">
             {ring && <RingDistributionChart data={ring} />}
          </CardContent>
        </Card>

        {/* Cluster Events / Logs (Placeholder for now) */}
        <Card className="h-full flex flex-col">
          <CardHeader>
            <CardTitle>Recent Events</CardTitle>
          </CardHeader>
          <CardContent className="flex-1">
            <div className="text-sm text-muted-foreground text-center py-10">
              No recent critical events
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
