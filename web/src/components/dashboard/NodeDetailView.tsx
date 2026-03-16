import { useEffect, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ArrowLeft, Server, RefreshCcw, WifiOff } from "lucide-react";
import { LineChart, Line, ResponsiveContainer, YAxis } from "recharts";
import { api } from "@/lib/api";
import { GossipMetrics, HealthResponse } from "@/lib/types";
import { useNodeHistory, NodeHistoryPoint } from "@/lib/useNodeHistory";

interface NodeDetailViewProps {
  nodeUrl: string;
  onBack?: () => void;
  showBackButton?: boolean;
}

export function NodeDetailView({ nodeUrl, onBack, showBackButton = true }: NodeDetailViewProps) {
  const [metrics, setMetrics] = useState<GossipMetrics | null>(null);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const history = useNodeHistory(health);

  useEffect(() => {
    let mounted = true;
    const fetch = async () => {
      try {
        const [g, h] = await Promise.all([
          api.getGossipMetrics(nodeUrl),
          api.getHealth(nodeUrl).catch(() => null),
        ]);
        if (mounted) { setMetrics(g); setHealth(h); }
      } finally {
        if (mounted) setLoading(false);
      }
    };
    fetch();
    const iv = setInterval(fetch, 2000);
    return () => { mounted = false; clearInterval(iv); };
  }, [nodeUrl]);

  if (loading && !metrics) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground space-y-3">
        <Server className="h-8 w-8 animate-pulse opacity-20" />
        <p>Connecting…</p>
      </div>
    );
  }

  const fmt = (s?: number) => {
    if (!s) return "0s";
    if (s < 60) return `${Math.floor(s)}s`;
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
    return h > 0 ? `${h}h ${m}m` : `${m}m`;
  };

  const isCompacting = health?.storage?.is_compacting;

  return (
    <div className="space-y-8 pb-10">

      {/* ── Header ─────────────────────────────────────────────────── */}
      <div className="flex items-start gap-4 border-b pb-5">
        {showBackButton && onBack && (
          <Button variant="ghost" size="icon" onClick={onBack} className="-ml-2 mt-0.5">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        )}
        <div className="flex-1">
          <div className="flex flex-wrap items-center gap-2 mb-1">
            <h2 className="text-xl font-bold tracking-tight font-mono">{nodeUrl}</h2>
            {health
              ? <Badge variant={health.status === "healthy" ? "outline" : "destructive"} className="text-xs uppercase">{health.status}</Badge>
              : <Badge variant="secondary" className="text-xs gap-1"><WifiOff className="h-2.5 w-2.5" />Unreachable</Badge>
            }
            {isCompacting && (
              <Badge className="text-xs gap-1 bg-amber-500/10 text-amber-600 border-amber-500/30 animate-pulse">
                <RefreshCcw className="h-2.5 w-2.5 animate-spin" />Compacting
              </Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            Uptime {fmt(health?.uptime_seconds)} · {metrics?.active_peers || 0} peers · v{health?.version || "—"}
          </p>
        </div>
      </div>

      {/* ── Top Metrics: Three Big Numbers ─────────────────────────── */}
      <div className="flex divide-x border rounded-xl overflow-hidden">
        <BigStat label="Requests / sec" value={health?.requests_per_second?.toFixed(1) || "0.0"} history={history} hkey="rps" />
        <BigStat label="Avg Latency" value={health?.average_latency_ms ? `${health.average_latency_ms.toFixed(1)} ms` : "0 ms"}
          history={history} hkey="latency_ms" alert={(health?.average_latency_ms || 0) > 100} />
        <BigStat label="Error Rate" value={health?.error_rate ? `${(health.error_rate * 100).toFixed(2)}%` : "0%"}
          alert={(health?.error_rate || 0) > 0.05} />
      </div>

      {/* ── Secondary Metrics Grid ─────────────────────────────────── */}
      <div className="grid grid-cols-3 gap-y-5 gap-x-4 py-4 border-y">
        <SmallStat label="GET / sec"   value={health?.gets_per_second?.toFixed(1) || "0.0"} />
        <SmallStat label="SET / sec"   value={health?.sets_per_second?.toFixed(1) || "0.0"} />
        <SmallStat label="DEL / sec"   value={health?.dels_per_second?.toFixed(1) || "0.0"} />
        <SmallStat label="Goroutines"  value={health?.goroutines_count?.toString() || "0"} />
        <SmallStat label="Heap Alloc"  value={health?.memory_allocated_mb ? `${health.memory_allocated_mb.toFixed(1)} MB` : "0 MB"} history={history} hkey="memory_mb" />
        <SmallStat label="GC Pauses"   value={health?.gc_pause_ms ? `${health.gc_pause_ms.toFixed(2)} ms` : "0 ms"} />
      </div>

      {/* ── LSM Storage ────────────────────────────────────────────── */}
      {health?.storage?.type === "lsm" && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-4">LSM Storage Engine</p>
          <div className="grid grid-cols-2 gap-y-5 gap-x-4">
            <SmallStat label="Disk Usage"       value={`${((health.storage.total_disk_usage_b || 0) / 1024).toFixed(2)} KB`} />
            <SmallStat label="SSTables"         value={health.storage.sstable_count || 0} />
            <SmallStat label="Keys (Approx)"    value={health.storage.approx_total_keys || 0} />
            <SmallStat label="MemTable Size"    value={`${((health.storage.memtable_size_b || 0) / 1024).toFixed(2)} KB`} />
            <SmallStat label="Compactions"      value={health.storage.compaction_count || 0} />
            <SmallStat label="Last Compaction"  value={health.storage.last_compaction_duration_ms ? `${health.storage.last_compaction_duration_ms} ms` : "N/A"} />
            <SmallStat label="Bloom FPR"        value={health.storage.bloom_false_positive_rate ? `${(health.storage.bloom_false_positive_rate * 100).toFixed(4)}%` : "0.00%"} />
            <SmallStat label="Flush Threshold"  value={`${((health.storage.flush_threshold_b || 0) / 1024).toFixed(0)} KB`} />
          </div>
        </div>
      )}

      {/* ── Tabs: Peers & Raw ──────────────────────────────────────── */}
      <Tabs defaultValue="peers">
        <TabsList className="mb-3">
          <TabsTrigger value="peers">Peers ({metrics?.total_peers || 0})</TabsTrigger>
          <TabsTrigger value="raw">Raw JSON</TabsTrigger>
        </TabsList>

        <TabsContent value="peers">
          <Card className="shadow-none border">
            <CardContent className="p-0">
              <ScrollArea className="h-[260px]">
                <Table>
                  <TableHeader className="bg-muted/50 sticky top-0">
                    <TableRow>
                      <TableHead>Peer URL</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Heartbeat</TableHead>
                      <TableHead className="text-right">Lag (ms)</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(metrics?.peer_details || []).map(peer => (
                      <TableRow key={peer.url}>
                        <TableCell className="font-mono text-xs">{peer.url}</TableCell>
                        <TableCell>
                          <Badge variant={peer.status === "active" ? "outline" : "destructive"}
                            className={peer.status === "active" ? "text-green-500 border-green-500/50" : ""}>
                            {peer.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right font-mono text-muted-foreground text-xs">{peer.heartbeat}</TableCell>
                        <TableCell className="text-right font-mono text-muted-foreground text-xs">{peer.lag}</TableCell>
                      </TableRow>
                    ))}
                    {!metrics?.peer_details?.length && (
                      <TableRow><TableCell colSpan={4} className="text-center h-20 text-muted-foreground">No peers found</TableCell></TableRow>
                    )}
                  </TableBody>
                </Table>
              </ScrollArea>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="raw">
          <Card className="shadow-none border">
            <CardContent className="p-0">
              <ScrollArea className="h-[360px] bg-muted/20">
                <pre className="p-4 text-xs font-mono text-muted-foreground">{JSON.stringify({ health, gossip: metrics }, null, 2)}</pre>
              </ScrollArea>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ── Visual Components ─────────────────────────────────────────────────

function BigStat({ label, value, history, hkey, alert }: {
  label: string;
  value: string;
  history?: NodeHistoryPoint[];
  hkey?: keyof NodeHistoryPoint;
  alert?: boolean;
}) {
  return (
    <div className="flex-1 px-5 py-4 flex flex-col gap-0.5">
      <p className="text-[10px] text-muted-foreground uppercase tracking-widest font-semibold">{label}</p>
      <p className={`text-3xl font-bold tracking-tight ${alert ? "text-destructive" : "text-foreground"}`}>{value}</p>
      {history && hkey && history.length > 1 && (
        <div className="h-[24px] mt-2 opacity-50">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={history}>
              <YAxis domain={["auto", "auto"]} hide />
              <Line type="monotone" dataKey={hkey} stroke={alert ? "#ef4444" : "#84cc16"} strokeWidth={2} dot={false} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

function SmallStat({ label, value, history, hkey }: {
  label: string;
  value: string | number;
  history?: NodeHistoryPoint[];
  hkey?: keyof NodeHistoryPoint;
}) {
  return (
    <div>
      <p className="text-xs text-muted-foreground mb-1">{label}</p>
      <p className="text-lg font-semibold">{value}</p>
      {history && hkey && history.length > 1 && (
        <div className="h-[20px] mt-1 opacity-40">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={history}>
              <YAxis domain={["auto", "auto"]} hide />
              <Line type="monotone" dataKey={hkey} stroke="#84cc16" strokeWidth={1.5} dot={false} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}
