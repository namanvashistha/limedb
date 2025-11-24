"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { GossipMetrics, PeerDetail } from "@/lib/types";
import { NodeSelector } from "./NodeSelector";
import { ClusterGrid } from "./ClusterGrid";
import { NodeDetails } from "./NodeDetails";
import { Card, CardContent } from "@/components/ui/card";
import { Loader2, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ClusterDashboardProps {
  onNavigate?: (tab: string) => void;
}

export function ClusterDashboard({ onNavigate }: ClusterDashboardProps) {
  const [selectedNode, setSelectedNode] = useState<string>("");
  const [metrics, setMetrics] = useState<GossipMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [allNodes, setAllNodes] = useState<string[]>([]);

  const fetchData = async () => {
    try {
      // Fetch metrics from the selected node (or default/seed if empty)
      const data = await api.getGossipMetrics(selectedNode);
      setMetrics(data);
      setError(null);

      // Update list of all known nodes
      const nodes = new Set<string>();
      if (selectedNode) nodes.add(selectedNode);
      
      // Add peers from the response
      if (data.peer_details) {
        data.peer_details.forEach(p => nodes.add(p.url));
      }
      
      // If we don't have a selected node yet, pick the first one or the one we just fetched from
      if (!selectedNode && nodes.size > 0) {
        // We might want to keep it empty to imply "Default/Seed", 
        // but for the UI it's better to be explicit.
        // However, if we are proxying, we need a valid URL.
        // If selectedNode is empty, we are talking to the seed/default node.
        // We should probably find out WHAT that node is.
        // But the API doesn't explicitly return "My URL" in the root of GossipMetrics,
        // it returns it in peer_details with lag=0 usually, or we can infer.
        // Actually, the backend `GetGossipMetrics` returns `node_heartbeat` but not the URL directly 
        // unless we look at the peer list where one might match?
        // Wait, the gossiper struct has `currentNodeUrl` but it's not in the root of the JSON response 
        // based on `gossiper.go`.
        // It returns `peer_details` which includes peers.
        // The `ClusterState` endpoint returns `nodeUrl`.
        // For now, let's just populate the list from peers.
      }

      setAllNodes(Array.from(nodes).sort());

    } catch (err) {
      // console.warn("Failed to fetch cluster data:", err);
      setError("Failed to connect to node. It may be down or the URL is incorrect.");
    } finally {
      setLoading(false);
    }
  };

  // Poll for updates
  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 2000);
    return () => clearInterval(interval);
  }, [selectedNode]);

  // Construct the peer list for the grid
  // We want to show ALL nodes.
  // The metrics.peer_details contains the peers of the *selectedNode*.
  // It does NOT contain the selectedNode itself in the list (usually).
  // Wait, `gossiper.go` `GetGossipMetrics` iterates over `peerHeartbeats`.
  // It does NOT include itself in `peerHeartbeats`.
  // So we need to manually add the selectedNode to the grid view if we want to see it.
  
  const gridPeers: PeerDetail[] = metrics ? [
    // The node we are querying (Observer)
    {
      url: selectedNode || "Current Node", // We might not know the URL if we haven't selected one explicitly
      heartbeat: metrics.node_heartbeat,
      lag: 0,
      status: "active"
    },
    ...(metrics.peer_details || [])
  ] : [];

  return (
    <div className="space-y-4 p-4">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <h2 className="text-2xl font-bold tracking-tight">Cluster Topology</h2>
          <NodeSelector 
            nodes={allNodes} 
            selectedNode={selectedNode} 
            onSelectNode={setSelectedNode} 
          />
        </div>
        <div className="flex items-center gap-2">
           {loading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
           <Button variant="ghost" size="icon" onClick={fetchData}>
             <RefreshCw className="h-4 w-4" />
           </Button>
        </div>
      </div>

      {error && (
        <Card className="border-destructive/50 bg-destructive/10">
          <CardContent className="pt-6 flex items-center justify-between gap-4 text-destructive">
            <span>{error}</span>
            {onNavigate && (
              <Button 
                variant="outline" 
                size="sm" 
                className="bg-background hover:bg-accent text-foreground border-destructive/20"
                onClick={() => onNavigate("settings")}
              >
                Configure Connection
              </Button>
            )}
          </CardContent>
        </Card>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2 space-y-4">
          <ClusterGrid 
            peers={gridPeers} 
            observerNode={selectedNode || "Current Node"} 
          />
        </div>
        <div className="lg:col-span-1">
          <NodeDetails metrics={metrics} nodeUrl={selectedNode || "Current Node"} />
        </div>
      </div>
    </div>
  );
}
