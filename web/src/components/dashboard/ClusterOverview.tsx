"use client";

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { KeyspaceMap } from "@/components/metrics/KeyspaceMap";
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

      {/* ── Node Grid / Heatmap ────────────────────────────────────── */}
      <div>
        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-3">Nodes Health</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
          {(ring?.allNodes ? [...ring.allNodes].sort() : []).map((nodeUrl) => {
            const h = clusterHealth.find(h => h.nodeUrl === nodeUrl);
            const isUp = !!h;
            return (
              <div key={nodeUrl} className="p-4 border rounded-xl bg-card flex flex-col gap-2 relative transition-all hover:border-primary/30">
                <div className="flex items-center gap-2">
                  <div className={`w-2 h-2 rounded-full flex-shrink-0 animate-pulse ${isUp ? "bg-green-500 shadow-[0_0_6px_#22c55e]" : "bg-muted-foreground/30"}`} />
                  <span className="font-mono text-xs font-semibold truncate flex-1">{nodeUrl.replace(/^https?:\/\//, "")}</span>
                  <Badge variant={isUp ? "outline" : "secondary"} className={`text-[10px] py-0 px-1.5 ${isUp ? "text-green-600 border-green-600/50" : ""}`}>
                    {isUp ? "up" : "down"}
                  </Badge>
                </div>

                {isUp ? (
                  <div className="flex flex-col gap-0.5">
                    <div className="flex justify-between">
                      <span className="text-[10px] text-muted-foreground">Requests</span>
                      <span className="text-xs font-bold tabular-nums">{(h.requests_per_second || 0).toFixed(1)} req/s</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-[10px] text-muted-foreground">Latency</span>
                      <span className="text-xs font-bold tabular-nums">{(h.average_latency_ms || 0).toFixed(1)} ms</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-[10px] text-muted-foreground">Memory</span>
                      <span className="text-xs font-bold tabular-nums">{(h.memory_allocated_mb || 0).toFixed(1)} MB</span>
                    </div>
                  </div>
                ) : (
                  <p className="text-xs text-muted-foreground h-10 flex items-center">Unreachable</p>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* ── Keyspace Map Visualization ────────────────────────────── */}
      <Card className="border shadow-none bg-muted/20">
        <CardContent className="p-6">
          {ring && <KeyspaceMap data={ring} />}
        </CardContent>
      </Card>
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
