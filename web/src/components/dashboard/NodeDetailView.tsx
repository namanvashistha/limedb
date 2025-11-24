import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ArrowLeft, Cpu, HardDrive, Activity } from "lucide-react";
import { api } from "@/lib/api";
import { GossipMetrics } from "@/lib/types";
import { NodeDetails } from "./NodeDetails"; // Reusing the existing component for gossip details

interface NodeDetailViewProps {
  nodeUrl: string;
  onBack?: () => void;
  showBackButton?: boolean;
}

export function NodeDetailView({ nodeUrl, onBack, showBackButton = true }: NodeDetailViewProps) {
  const [metrics, setMetrics] = useState<GossipMetrics | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        // In a real scenario, we might want to fetch specific node metrics here
        // For now, we'll use the global gossip metrics and filter/display relevant info
        // Ideally, the backend would support /api/v1/node/:id/metrics
        const data = await api.getGossipMetrics(nodeUrl);
        setMetrics(data);
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

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        {showBackButton && onBack && (
          <Button variant="outline" size="icon" onClick={onBack}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
        )}
        <h2 className="text-2xl font-bold tracking-tight">{nodeUrl}</h2>
        {metrics && (
           <Badge variant={metrics.cluster_health === "healthy" ? "default" : "destructive"}>
             {metrics.cluster_health.toUpperCase()}
           </Badge>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <MetricCard 
          title="CPU Usage" 
          value={`${(Math.random() * 5 + 1).toFixed(1)}%`} 
          icon={Cpu} 
          subtext="Mocked Data"
        />
        <MetricCard 
          title="Memory Usage" 
          value={`${(Math.random() * 200 + 100).toFixed(0)} MB`} 
          icon={HardDrive} 
          subtext="Mocked Data"
        />
        <MetricCard 
          title="Request Latency" 
          value={`${(Math.random() * 10).toFixed(2)} ms`} 
          icon={Activity} 
          subtext="Mocked Data"
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
