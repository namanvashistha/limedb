import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ArrowLeft, Cpu, HardDrive, Activity } from "lucide-react";
import { api } from "@/lib/api";
import { GossipMetrics, HealthResponse } from "@/lib/types";
import { NodeDetails } from "./NodeDetails"; // Reusing the existing component for gossip details

interface NodeDetailViewProps {
  nodeUrl: string;
  onBack?: () => void;
  showBackButton?: boolean;
}

export function NodeDetailView({ nodeUrl, onBack, showBackButton = true }: NodeDetailViewProps) {
  const [metrics, setMetrics] = useState<GossipMetrics | null>(null);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const [gossipData, healthData] = await Promise.all([
          api.getGossipMetrics(nodeUrl),
          api.getHealth(nodeUrl).catch(() => null) // fail gracefully if health is unavailable
        ]);
        
        setMetrics(gossipData);
        setHealth(healthData);
      } catch (err) {
        // console.warn("Failed to fetch node metrics", err);
      } finally {
        setLoading(false);
      }
    };

    fetchMetrics();
    const interval = setInterval(fetchMetrics, 2000);
    return () => clearInterval(interval);
  }, [nodeUrl]);

  if (loading) {
    return <div>Loading node details...</div>;
  }

  const formatUptime = (seconds?: number) => {
    if (seconds === undefined) return "Unknown";
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) return `${h}h ${m}m`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        {showBackButton && onBack && (
          <Button variant="outline" size="icon" onClick={onBack}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
        )}
        <h2 className="text-2xl font-bold tracking-tight">{nodeUrl}</h2>
        {health && (
           <Badge variant={health.status === "healthy" ? "default" : "destructive"}>
             {health.status.toUpperCase()}
           </Badge>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <MetricCard 
          title="Uptime" 
          value={formatUptime(health?.uptime_seconds)} 
          icon={Activity} 
          subtext="Go runtime"
        />
        <MetricCard 
          title="Memory Allocated" 
          value={health?.memory_allocated_mb ? `${health.memory_allocated_mb.toFixed(2)} MB` : "Unknown"} 
          icon={HardDrive} 
          subtext="Go runtime heap"
        />
        <MetricCard 
          title="Active Goroutines" 
          value={health?.goroutines_count?.toString() || "Unknown"} 
          icon={Cpu} 
          subtext="Concurrency threads"
        />
      </div>

      <Tabs defaultValue="gossip" className="w-full">
        <TabsList>
          <TabsTrigger value="gossip">Gossip Protocol</TabsTrigger>
          <TabsTrigger value="raw">Raw State</TabsTrigger>
        </TabsList>
        <TabsContent value="gossip" className="mt-4">
          <NodeDetails metrics={metrics} nodeUrl={nodeUrl} />
        </TabsContent>
        <TabsContent value="raw" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>Raw Node State</CardTitle>
            </CardHeader>
            <CardContent>
              <ScrollArea className="h-[400px] w-full rounded-md border bg-muted/50 p-4">
                <pre className="text-xs font-mono">
                  {JSON.stringify(metrics, null, 2)}
                </pre>
              </ScrollArea>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function MetricCard({ title, value, icon: Icon, subtext }: { title: string; value: string; icon: any; subtext?: string }) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        {subtext && <p className="text-xs text-muted-foreground">{subtext}</p>}
      </CardContent>
    </Card>
  );
}
