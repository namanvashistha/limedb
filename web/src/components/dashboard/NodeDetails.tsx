import { useEffect, useState } from "react";
import { GossipMetrics, HealthResponse } from "@/lib/types";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

interface NodeDetailsProps {
  metrics: GossipMetrics | null;
  nodeUrl: string;
}

export function NodeDetails({ metrics, nodeUrl }: NodeDetailsProps) {
  const [health, setHealth] = useState<HealthResponse | null>(null);

  useEffect(() => {
    if (!nodeUrl) return;
    
    const fetchHealth = async () => {
      try {
        const data = await api.getHealth(nodeUrl);
        setHealth(data);
      } catch (err) {
        console.error("Failed to fetch node health:", err);
      }
    };
    
    fetchHealth();
    const interval = setInterval(fetchHealth, 2000);
    return () => clearInterval(interval);
  }, [nodeUrl]);

  if (!metrics) {
    return (
      <Card className="h-full">
        <CardHeader>
          <CardTitle>Node Details</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center h-40 text-muted-foreground">
            Select a node to view details
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="h-full flex flex-col">
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span>Details: {nodeUrl}</span>
          <Badge variant={metrics.cluster_health === "healthy" ? "default" : "destructive"}>
            {metrics.cluster_health.toUpperCase()}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex-1 overflow-hidden">
        <Tabs defaultValue="overview" className="h-full flex flex-col">
          <TabsList className="w-full justify-start">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="storage">Storage</TabsTrigger>
            <TabsTrigger value="peers">Peers ({metrics.total_peers})</TabsTrigger>
            <TabsTrigger value="raw">Raw JSON</TabsTrigger>
          </TabsList>
          
          <TabsContent value="overview" className="flex-1 overflow-auto mt-4">
            <div className="grid grid-cols-2 gap-4">
              <MetricCard label="Heartbeat" value={metrics.node_heartbeat} />
              <MetricCard label="Active Peers" value={`${metrics.active_peers}/${metrics.total_peers}`} />
              <MetricCard label="Convergence" value={`${(metrics.convergence_rate || 0).toFixed(1)}%`} />
              <MetricCard label="Avg Lag" value={(metrics.average_lag || 0).toFixed(2)} />
              <MetricCard label="Max Lag" value={metrics.max_lag} />
              <MetricCard label="Dead Peers" value={metrics.dead_peers} alert={metrics.dead_peers > 0} />
            </div>
          </TabsContent>

          <TabsContent value="storage" className="flex-1 overflow-auto mt-4">
            <div className="grid grid-cols-2 gap-4">
              <MetricCard label="Engine" value={health?.storage?.type || "Unknown"} />
              {health?.storage?.type === 'lsm' ? (
                <>
                  <MetricCard label="MemTable Size" value={`${(health.storage.memtable_size_b! / 1024).toFixed(2)} KB`} />
                  <MetricCard label="MemTable Keys" value={health.storage.memtable_keys || 0} />
                  <MetricCard label="SSTables" value={health.storage.sstable_count || 0} />
                  <MetricCard label="Flush Threshold" value={`${(health.storage.flush_threshold_b! / (1024 * 1024)).toFixed(2)} MB`} />
                  <MetricCard label="Compactions" value={health.storage.compaction_count || 0} />
                  <MetricCard label="Total Disk Usage" value={`${((health.storage.total_disk_usage_b || 0) / 1024).toFixed(2)} KB`} />
                  <MetricCard label="Total Keys (Approx)" value={health.storage.approx_total_keys || 0} />
                  <MetricCard label="Bloom FPR" value={health.storage.bloom_false_positive_rate ? `${(health.storage.bloom_false_positive_rate * 100).toFixed(4)}%` : "0.00%"} />
                </>
              ) : (
                <MetricCard label="Total Keys" value={health?.storage?.keys || 0} />
              )}
            </div>
          </TabsContent>

          <TabsContent value="peers" className="flex-1 overflow-hidden mt-4">
            <ScrollArea className="h-[400px] w-full rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Peer URL</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Heartbeat</TableHead>
                    <TableHead className="text-right">Lag</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(metrics.peer_details || []).map((peer) => (
                    <TableRow key={peer.url}>
                      <TableCell className="font-mono text-xs">{peer.url}</TableCell>
                      <TableCell>
                        <Badge 
                          variant={peer.status === "active" ? "outline" : "destructive"}
                          className={peer.status === "active" ? "text-green-500 border-green-500" : ""}
                        >
                          {peer.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right font-mono">{peer.heartbeat}</TableCell>
                      <TableCell className="text-right font-mono">{peer.lag}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </ScrollArea>
          </TabsContent>

          <TabsContent value="raw" className="flex-1 overflow-hidden mt-4">
            <ScrollArea className="h-[400px] w-full rounded-md border bg-muted/50 p-4">
              <pre className="text-xs font-mono">
                {JSON.stringify(metrics, null, 2)}
              </pre>
            </ScrollArea>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}

function MetricCard({ label, value, alert }: { label: string; value: string | number; alert?: boolean }) {
  return (
    <div className={`p-4 rounded-lg border ${alert ? 'bg-destructive/10 border-destructive' : 'bg-card'}`}>
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className={`text-2xl font-bold ${alert ? 'text-destructive' : ''}`}>{value}</div>
    </div>
  );
}
