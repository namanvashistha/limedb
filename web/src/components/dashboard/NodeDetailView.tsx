import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ArrowLeft, Cpu, HardDrive, Activity, BarChart3, AlertTriangle, Timer, Clock, Database, Server, RefreshCcw, WifiOff } from "lucide-react";
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
    const fetchMetrics = async () => {
      try {
        const [gossipData, healthData] = await Promise.all([
          api.getGossipMetrics(nodeUrl),
          api.getHealth(nodeUrl).catch(() => null) // fail gracefully
        ]);
        
        if (mounted) {
          setMetrics(gossipData);
          setHealth(healthData);
        }
      } catch (err) {
        console.error("Failed to fetch node metrics", err);
      } finally {
        if (mounted) setLoading(false);
      }
    };

    fetchMetrics();
    const interval = setInterval(fetchMetrics, 2000);
    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, [nodeUrl]);

  if (loading && !metrics) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground space-y-4">
        <Server className="h-8 w-8 animate-pulse text-muted" />
        <p>Connecting to Node...</p>
      </div>
    );
  }

  const formatUptime = (seconds?: number) => {
    if (seconds === undefined) return "Unknown";
    if (seconds < 60) return `${Math.floor(seconds)}s`;
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return h > 0 ? `${h}h ${m}m` : `${m}m`;
  };

  const isCompacting = health?.storage?.is_compacting;

  return (
    <div className="space-y-6 pb-10">
      {/* Header Row */}
      <div className="flex items-center gap-4 border-b pb-4">
        {showBackButton && onBack && (
          <Button variant="outline" size="icon" onClick={onBack} className="flex-shrink-0">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        )}
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h2 className="text-2xl font-bold tracking-tight">{nodeUrl}</h2>
            {health && (
              <Badge variant={health.status === "healthy" ? "default" : "destructive"} className="uppercase">
                {health.status}
              </Badge>
            )}
            {!health && !loading && (
              <Badge variant="destructive" className="uppercase flex items-center gap-1">
                <WifiOff className="h-3 w-3" /> Unreachable
              </Badge>
            )}
            {metrics?.is_seed && (
              <Badge variant="secondary" className="uppercase bg-blue-500/10 text-blue-600 border-blue-500/20">
                Seed Node
              </Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground mt-1 flex items-center gap-4">
            <span className="flex items-center gap-1.5"><Activity className="h-3.5 w-3.5" /> Uptime: {formatUptime(health?.uptime_seconds)}</span>
            <span className="flex items-center gap-1.5"><Server className="h-3.5 w-3.5" /> Gossip: {metrics?.active_peers || 0} Peers</span>
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        
        {/* Left Column: Network & Routing */}
        <div className="xl:col-span-1 space-y-6">
          <div className="space-y-3">
            <h3 className="text-md font-semibold text-foreground/80 flex items-center gap-2">
              <BarChart3 className="h-4 w-4" /> Routing & Throughput
            </h3>
            <div className="grid grid-cols-2 gap-3">
              <MiniCard label="Total Req/sec" value={health?.requests_per_second?.toFixed(1) || "0.0"} sparklineData={history} sparklineKey="rps" />
              <MiniCard label="Avg Latency" value={health?.average_latency_ms ? `${health.average_latency_ms.toFixed(2)} ms` : "0 ms"} alert={(health?.average_latency_ms || 0) > 100} sparklineData={history} sparklineKey="latency_ms" />
              
              <MiniCard label="GET/sec" value={health?.gets_per_second?.toFixed(1) || "0.0"} />
              <MiniCard label="SET/sec" value={health?.sets_per_second?.toFixed(1) || "0.0"} />
              
              <MiniCard label="DEL/sec" value={health?.dels_per_second?.toFixed(1) || "0.0"} />
              <MiniCard label="4xx / 5xx Err" value={health?.error_rate ? `${(health.error_rate * 100).toFixed(2)}%` : "0.00%"} alert={(health?.error_rate || 0) > 0.05} />
            </div>
          </div>

          <div className="space-y-3">
            <h3 className="text-md font-semibold text-foreground/80 flex items-center gap-2">
              <Cpu className="h-4 w-4" /> Go Runtime
            </h3>
            <div className="grid grid-cols-2 gap-3">
              <MiniCard label="Sys RAM Used" value={health?.memory_sys_mb ? `${health.memory_sys_mb.toFixed(1)} MB` : "0 MB"} sparklineData={history} sparklineKey="memory_mb" />
              <MiniCard label="Goroutines" value={health?.goroutines_count?.toString() || "0"} />
              <MiniCard label="Heap Alloc" value={health?.memory_allocated_mb ? `${health.memory_allocated_mb.toFixed(1)} MB` : "0 MB"} />
              <MiniCard label="GC Pauses" value={health?.gc_pause_ms ? `${health.gc_pause_ms.toFixed(2)} ms` : "0 ms"} />
            </div>
          </div>
        </div>

        {/* Right Column: Storage Engine */}
        <div className="xl:col-span-2 space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-md font-semibold text-foreground/80 flex items-center gap-2">
              <Database className="h-4 w-4" /> LSM Storage Engine
            </h3>
            {isCompacting && (
              <Badge className="bg-amber-500/20 text-amber-600 border-amber-500/30 animate-pulse flex items-center gap-1.5">
                <RefreshCcw className="h-3 w-3 animate-spin"/> Compacting
              </Badge>
            )}
          </div>
          
          <Card className={`border-2 shadow-sm ${isCompacting ? 'border-amber-500/50' : 'border-border'}`}>
            <CardContent className="p-5">
              <div className="grid grid-cols-2 md:grid-cols-4 gap-y-6 gap-x-4">
                <MetricBlock label="Total Disk Usage" value={`${((health?.storage?.total_disk_usage_b || 0) / 1024).toFixed(2)} KB`} />
                <MetricBlock label="Active SSTables" value={health?.storage?.sstable_count || 0} />
                <MetricBlock label="Keys (Approx)" value={health?.storage?.approx_total_keys || 0} />
                <MetricBlock label="MemTable Size" value={`${((health?.storage?.memtable_size_b || 0) / 1024).toFixed(2)} KB`} />
                
                <div className="col-span-4 h-px bg-border my-2" />

                <MetricBlock label="Total Compactions" value={health?.storage?.compaction_count || 0} />
                <MetricBlock label="Last Compaction" value={health?.storage?.last_compaction_duration_ms ? `${health.storage.last_compaction_duration_ms} ms` : "N/A"} />
                <MetricBlock label="Bloom FPR" value={health?.storage?.bloom_false_positive_rate ? `${(health.storage.bloom_false_positive_rate * 100).toFixed(4)}%` : "0.00%"} />
                <MetricBlock label="Mem Flush Thresh" value={`${((health?.storage?.flush_threshold_b || 0) / 1024).toFixed(0)} KB`} />
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      <div className="mt-8">
        <Tabs defaultValue="peers" className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="peers">Connected Peers ({metrics?.total_peers || 0})</TabsTrigger>
            <TabsTrigger value="raw">Raw State JSON</TabsTrigger>
          </TabsList>
          
          <TabsContent value="peers">
            <Card>
              <CardContent className="p-0">
                <ScrollArea className="h-[300px] w-full">
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
                      {(metrics?.peer_details || []).map((peer) => (
                        <TableRow key={peer.url}>
                          <TableCell className="font-mono text-xs">{peer.url}</TableCell>
                          <TableCell>
                            <Badge variant={peer.status === "active" ? "outline" : "destructive"} className={peer.status === "active" ? "text-green-500 border-green-500" : ""}>
                              {peer.status}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right font-mono text-muted-foreground">{peer.heartbeat}</TableCell>
                          <TableCell className="text-right font-mono text-muted-foreground">{peer.lag}</TableCell>
                        </TableRow>
                      ))}
                      {(!metrics?.peer_details || metrics.peer_details.length === 0) && (
                        <TableRow>
                          <TableCell colSpan={4} className="text-center h-24 text-muted-foreground">
                            No peers connected to this node
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                </ScrollArea>
              </CardContent>
            </Card>
          </TabsContent>
          
          <TabsContent value="raw">
            <Card>
              <CardContent className="p-0">
                <ScrollArea className="h-[400px] w-full bg-muted/30">
                  <pre className="p-4 text-xs font-mono text-muted-foreground">
                    {JSON.stringify({ health, gossip: metrics }, null, 2)}
                  </pre>
                </ScrollArea>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}

// Visual Helpers
function MiniCard({ label, value, alert, sparklineData, sparklineKey }: { 
  label: string; 
  value: string | number; 
  alert?: boolean | null;
  sparklineData?: NodeHistoryPoint[];
  sparklineKey?: keyof NodeHistoryPoint;
}) {
  return (
    <div className={`p-3 rounded-lg border flex flex-col justify-between ${alert ? 'bg-destructive/10 border-destructive shadow-[inset_0_0_0_1px_rgba(239,68,68,0.2)]' : 'bg-card'}`}>
      <div>
        <div className="text-xs text-muted-foreground mb-1">{label}</div>
        <div className={`text-xl font-bold tracking-tight ${alert ? 'text-destructive' : 'text-foreground'}`}>{value}</div>
      </div>
      {sparklineData && sparklineKey && sparklineData.length > 1 && (
        <div className="h-[30px] mt-3 w-full opacity-60">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={sparklineData}>
              <YAxis domain={['auto', 'auto']} hide />
              <Line 
                type="monotone" 
                dataKey={sparklineKey} 
                stroke={alert ? "#ef4444" : "#84cc16"} 
                strokeWidth={2} 
                dot={false} 
                isAnimationActive={false} 
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

function MetricBlock({ label, value }: { label: string; value: string | number }) {
  return (
    <div>
      <div className="text-xs text-muted-foreground mb-1 font-medium">{label}</div>
      <div className="text-xl font-semibold">{value}</div>
    </div>
  );
}
