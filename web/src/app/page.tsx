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
  const [node, setNode] = useState<NodeStatus | null>(null);
  const [ring, setRing] = useState<RingState>({ ranges: {} });
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [nodeData, ringData] = await Promise.all([
        api.getClusterStatus(),
        api.getRingState(),
      ]);
      setNode(nodeData);
      setRing(ringData);
      setLastUpdated(new Date());
    } catch (err) {
      console.error("Failed to fetch data:", err);
      setError(err instanceof Error ? err.message : "Failed to connect to cluster");
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
  const activeNodes = node?.status === "active" ? 1 : 0;
  const totalNodes = node ? 1 + (node.peers?.length || 0) : 0;
  
  let health = "unknown";
  if (node) {
    health = node.status === "active" ? "healthy" : "critical";
  }

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

        {/* Error Message */}
        {error && (
          <div className="rounded-md border border-red-500 bg-red-500/10 p-4 text-red-500">
            {error}
          </div>
        )}

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
                <ClusterStatusTable node={node} />
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
