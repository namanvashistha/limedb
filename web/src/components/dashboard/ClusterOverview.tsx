"use client";

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { RingDistributionChart } from "@/components/metrics/RingDistributionChart";
import { api } from "@/lib/api";
import { RingState, GossipMetrics, HealthResponse } from "@/lib/types";
import { AlertCircle } from "lucide-react";

export function ClusterOverview() {
  const [ring, setRing] = useState<RingState | null>(null);
  const [gossip, setGossip] = useState<GossipMetrics | null>(null);
  const [clusterHealth, setClusterHealth] = useState<HealthResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    const fetchData = async () => {
      try {
        const [gossipData, ringData] = await Promise.all([
          api.getGossipMetrics(),
          api.getRingState(),
        ]);

        if (!mounted) return;

        if (gossipData) {
          const allUrls = [gossipData.node_url, ...(gossipData.peer_details?.map(p => p.url) || [])];
          const healthResults = await Promise.all(allUrls.map(url => api.getHealth(url).catch(() => null)));
          if (mounted) setClusterHealth(healthResults.filter(Boolean) as HealthResponse[]);
        }

        setGossip(gossipData);
        setRing(ringData);
        setError(null);
      } catch {
        setError("Failed to connect to cluster");
      } finally {
        if (mounted) setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 2000);
    return () => { mounted = false; clearInterval(interval); };
  }, []);

  if (loading && !ring) {
    return <div className="flex items-center justify-center h-64 text-muted-foreground">Connecting to cluster...</div>;
  }

  if (error && !ring) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-destructive gap-3">
        <AlertCircle className="h-10 w-10 opacity-50" />
        <p className="font-medium">{error}</p>
      </div>
    );
  }

  const totalNodes = ring?.allNodes?.length || (gossip ? gossip.total_peers + 1 : 0);
  const healthyNodes = gossip ? gossip.active_peers + 1 : 0;
  const deadNodes = gossip?.dead_peers || 0;
  const totalRps = clusterHealth.reduce((sum, h) => sum + (h.requests_per_second || 0), 0);
  const totalDiskKb = clusterHealth.reduce((sum, h) => sum + ((h.storage?.total_disk_usage_b || 0) / 1024), 0);
  const activeRanges = ring?.ranges ? Object.values(ring.ranges).flat().length : 0;
  const isHealthy = gossip?.cluster_health === "healthy";

  return (
    <div className="space-y-8">
      {/* ── Borderless Stat Strip ─────────────────────────────────── */}
      <div className="flex flex-wrap items-stretch gap-0 border rounded-xl overflow-hidden bg-card divide-x">
        <StatCell
          label="Nodes"
          value={String(totalNodes)}
          sub={<><span className="text-green-500">{healthyNodes} healthy</span>{deadNodes > 0 && <span className="text-destructive ml-1">{deadNodes} dead</span>}</>}
        />
        <StatCell
          label="Cluster Status"
          value={gossip?.cluster_health || "—"}
          sub={`${(gossip?.convergence_rate || 0).toFixed(1)}% convergence`}
          accent={isHealthy ? "green" : "red"}
        />
        <StatCell
          label="Global RPS"
          value={`${totalRps.toFixed(1)}`}
          sub="requests / sec across cluster"
          accent="lime"
        />
        <StatCell
          label="Total Disk"
          value={`${totalDiskKb.toFixed(1)} KB`}
          sub="SSTables on disk"
        />
        <StatCell
          label="Hash Ranges"
          value={String(activeRanges)}
          sub="active token ranges"
        />
      </div>

      {/* ── Ring Visualization ─────────────────────────────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6" style={{ height: 460 }}>
        <Card className="lg:col-span-2 h-full flex flex-col border-0 shadow-none bg-muted/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-base font-medium text-foreground/70">Token Ring Distribution</CardTitle>
          </CardHeader>
          <CardContent className="flex-1 min-h-0">
            {ring && <RingDistributionChart data={ring} />}
          </CardContent>
        </Card>

        {/* Node Health Column */}
        <div className="flex flex-col gap-3 h-full overflow-y-auto">
          <p className="text-sm font-medium text-foreground/60 mb-1">Node Health</p>
          {(ring?.allNodes || []).map((nodeUrl, i) => {
            const h = clusterHealth.find(h => h.nodeUrl === nodeUrl);
            const isUp = !!h;
            return (
              <div key={nodeUrl} className="flex items-center gap-3 py-2 border-b last:border-0">
                <div className={`w-2 h-2 rounded-full flex-shrink-0 ${isUp ? "bg-green-500" : "bg-muted-foreground/30"}`} />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-mono truncate">{nodeUrl}</p>
                  {isUp && <p className="text-xs text-muted-foreground">{(h.requests_per_second || 0).toFixed(1)} req/s · {(h.memory_allocated_mb || 0).toFixed(1)} MB</p>}
                </div>
                <Badge variant={isUp ? "outline" : "secondary"} className={`text-xs ${isUp ? "text-green-600 border-green-600/50" : "text-muted-foreground"}`}>
                  {isUp ? "up" : "down"}
                </Badge>
              </div>
            );
          })}
          {(!ring?.allNodes || ring.allNodes.length === 0) && (
            <p className="text-sm text-muted-foreground py-4">No nodes discovered yet</p>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Helpers ───────────────────────────────────────────────────────────
function StatCell({
  label,
  value,
  sub,
  accent,
}: {
  label: string;
  value: string;
  sub: React.ReactNode;
  accent?: "green" | "red" | "lime";
}) {
  const accentColor =
    accent === "green" ? "text-green-500" :
    accent === "red"   ? "text-destructive" :
    accent === "lime"  ? "text-lime-500" :
    "text-foreground";

  return (
    <div className="flex-1 min-w-[140px] px-6 py-5">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-1">{label}</p>
      <p className={`text-3xl font-bold tracking-tight ${accentColor}`}>{value}</p>
      <p className="text-xs text-muted-foreground mt-1">{sub}</p>
    </div>
  );
}
