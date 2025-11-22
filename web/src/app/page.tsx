"use client";

import { useEffect, useState } from "react";
import { SummaryCards } from "@/components/dashboard/SummaryCards";
import { ClusterStatusTable } from "@/components/dashboard/ClusterStatusTable";
import { RingVisualizer } from "@/components/dashboard/RingVisualizer";
import { DataExplorer } from "@/components/dashboard/DataExplorer";
import { api } from "@/lib/api";
import { NodeStatus, RingState } from "@/lib/types";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function Dashboard() {
  const [nodes, setNodes] = useState<Record<string, NodeStatus>>({});
  const [ring, setRing] = useState<RingState>({ ranges: {}, version: 0 });
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [loading, setLoading] = useState(false);

  const fetchData = async () => {
    setLoading(true);
    try {
      const [nodesData, ringData] = await Promise.all([
        api.getClusterStatus(),
        api.getRingState(),
      ]);
      setNodes(nodesData);
      setRing(ringData);
      setLastUpdated(new Date());
    } catch (error) {
      console.error("Failed to fetch data:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 2000);
    return () => clearInterval(interval);
  }, []);

  // Calculate summary stats
  const activeNodes = Object.values(nodes).filter(
    (n) => n.status === "active"
  ).length;
  const totalNodes = Object.keys(nodes).length;
  
  let health = "healthy";
  if (activeNodes === 0) health = "critical";
  else if (activeNodes < totalNodes) health = "degraded";

  let totalKeys = 0;
  if (ring.ranges) {
    Object.values(ring.ranges).forEach((ranges) => {
      ranges.forEach((r) => (totalKeys += r.size));
    });
  }

  return (
    <div className="min-h-screen bg-background p-8">
      <div className="mx-auto max-w-7xl space-y-8">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight text-lime-500">LimeDB Dashboard</h1>
            <p className="text-muted-foreground">
              Cluster Overview & Data Explorer
            </p>
          </div>
          <div className="flex items-center gap-4">
            <span className="text-sm text-muted-foreground">
              Last updated: {lastUpdated.toLocaleTimeString()}
            </span>
            <Button variant="outline" size="icon" onClick={fetchData} disabled={loading}>
              <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            </Button>
          </div>
        </div>

        {/* Summary */}
        <SummaryCards
          health={health}
          activeNodes={activeNodes}
          totalKeys={totalKeys}
        />

        {/* Main Content */}
        <Tabs defaultValue="overview" className="space-y-4">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="explorer">Data Explorer</TabsTrigger>
          </TabsList>
          
          <TabsContent value="overview" className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-7">
              <div className="col-span-4">
                <ClusterStatusTable nodes={nodes} />
              </div>
              <div className="col-span-3">
                <RingVisualizer ring={ring} />
              </div>
            </div>
          </TabsContent>
          
          <TabsContent value="explorer">
            <DataExplorer />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
