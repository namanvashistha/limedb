"use client";

import { useEffect, useState, useRef } from "react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { SummaryCards } from "@/components/dashboard/SummaryCards";
import { ClusterDashboard } from "@/components/dashboard/ClusterDashboard";
import { EnhancedDataExplorer } from "@/components/dashboard/EnhancedDataExplorer";
import { PerformanceCharts } from "@/components/metrics/PerformanceCharts";
import { RingDistributionChart } from "@/components/metrics/RingDistributionChart";
import { SettingsPanel } from "@/components/settings/SettingsPanel";
import { CommandPalette } from "@/components/command-palette";
import { NetworkTopology } from "@/components/topology/NetworkTopology";
import { api } from "@/lib/api";
import { NodeStatus, RingState, GossipMetrics } from "@/lib/types";
import { RefreshCw, Clock, Wifi, WifiOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { motion, AnimatePresence } from "framer-motion";
import { Badge } from "@/components/ui/badge";

export default function Dashboard() {
  const [nodes, setNodes] = useState<Record<string, NodeStatus>>({});
  const [ring, setRing] = useState<RingState>({ ranges: {} });
  const [gossip, setGossip] = useState<GossipMetrics | null>(null);
  const [discoveredNodes, setDiscoveredNodes] = useState<string[]>([]);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState("overview");
  const [connectionStatus, setConnectionStatus] = useState<"connected" | "disconnected" | "connecting">("connecting");
  const [heartbeatChanges, setHeartbeatChanges] = useState<Record<string, number>>({});
  
  // Settings state
  const [refreshInterval, setRefreshInterval] = useState(2000);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    setConnectionStatus("connecting");
    
    try {
      // First discover all nodes
      const hosts = await api.discoverCluster();
      setDiscoveredNodes(hosts);
      
      // Use gossip as source of truth for cluster status - NO SELF NODE CONCEPT
      const [clusterData, ringData] = await Promise.all([
        api.getClusterStatus(), // Returns { gossip, nodes } - all nodes equal
        api.getRingState(),
      ]);
      
      // Track heartbeat changes for animation
      const newHeartbeatChanges: Record<string, number> = {};
      Object.entries(clusterData.nodes).forEach(([url, node]) => {
        const oldNode = nodes[url];
        if (oldNode && node.status === "active") {
          // Check if heartbeat increased (node is alive and updating)
          newHeartbeatChanges[url] = Date.now();
        }
      });
      setHeartbeatChanges(newHeartbeatChanges);
      
      setNodes(clusterData.nodes);
      setGossip(clusterData.gossip);
      setRing(ringData);
      setLastUpdated(new Date());
      setConnectionStatus("connected");
    } catch (err) {
      console.error("Failed to fetch data:", err);
      setError(err instanceof Error ? err.message : "Failed to connect to cluster");
      setConnectionStatus("disconnected");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    if (!autoRefresh) return;
    
    const interval = setInterval(fetchData, refreshInterval);
    return () => clearInterval(interval);
  }, [refreshInterval, autoRefresh]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyboard = (e: KeyboardEvent) => {
      if (e.key === "r" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        fetchData();
      }
      if (e.key >= "1" && e.key <= "4" && !e.metaKey && !e.ctrlKey) {
        const tabs = ["overview", "metrics", "explorer", "settings"];
        setActiveTab(tabs[parseInt(e.key) - 1]);
      }
    };

    document.addEventListener("keydown", handleKeyboard);
    return () => document.removeEventListener("keydown", handleKeyboard);
  }, []);

  // Calculate summary stats from gossip-derived nodes
  const nodesList = Object.values(nodes);
  const activeNodes = nodesList.filter(n => n.status === "active").length;
  const totalNodes = discoveredNodes.length || nodesList.length;
  
  let health = "unknown";
  if (activeNodes === 0 && totalNodes > 0) {
    health = "critical";
  } else if (activeNodes < totalNodes) {
    health = "degraded";
  } else if (activeNodes > 0) {
    health = "healthy";
  }

  let totalKeys = 0;
  if (ring.ranges) {
    Object.values(ring.ranges).forEach((ranges) => {
      ranges.forEach((r) => (totalKeys += r.size));
    });
  }

  // Calculate time since last update
  const getTimeSinceUpdate = () => {
    const seconds = Math.floor((Date.now() - lastUpdated.getTime()) / 1000);
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    return `${minutes}m ago`;
  };

  return (
    <DashboardLayout 
      activeTab={activeTab} 
      onTabChange={setActiveTab}
      headerAction={
        <div className="flex items-center gap-3">
           {/* Connection Status */}
           <Badge 
             variant={connectionStatus === "connected" ? "default" : "destructive"}
             className={`gap-1.5 ${connectionStatus === "connected" ? "bg-green-600 hover:bg-green-700" : ""}`}
           >
             {connectionStatus === "connected" ? (
               <>
                 <div className="h-2 w-2 rounded-full bg-white animate-pulse" />
                 Live
               </>
             ) : connectionStatus === "connecting" ? (
               <>
                 <div className="h-2 w-2 rounded-full bg-white animate-spin" />
                 Connecting
               </>
             ) : (
               <>
                 <WifiOff className="h-3 w-3" />
                 Offline
               </>
             )}
           </Badge>

           {/* Last Update Time */}
           <div className="text-xs text-muted-foreground hidden sm:flex items-center gap-2" suppressHydrationWarning>
              <Clock className="h-3 w-3" />
              <span suppressHydrationWarning>
                {getTimeSinceUpdate()}
              </span>
           </div>

           {/* Refresh Button */}
           <Button
              variant="outline"
              size="sm"
              onClick={fetchData}
              disabled={loading}
              className="gap-2"
            >
              <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
              <span className="hidden sm:inline">Refresh</span>
            </Button>
            <CommandPalette onNavigate={setActiveTab} onRefresh={fetchData} />
        </div>
      }
    >
      {/* Animated Error Message */}
      <AnimatePresence>
        {error && (
          <motion.div
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
          >
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          </motion.div>
        )}
      </AnimatePresence>

      <AnimatePresence mode="wait">
        <motion.div
          key={activeTab}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
          transition={{ duration: 0.2 }}
        >
          {activeTab === "overview" && (
            <div className="space-y-6">
               <SummaryCards
                health={health}
                activeNodes={activeNodes}
                totalNodes={totalNodes}
                totalKeys={totalKeys}
              />
              <ClusterDashboard onNavigate={setActiveTab} />
            </div>
          )}
          
          {activeTab === "metrics" && (
            <div className="space-y-6">
              <div className="grid gap-6 lg:grid-cols-2">
                <NetworkTopology nodes={nodes} />
                <RingDistributionChart ring={ring} gossip={gossip} />
              </div>
              <PerformanceCharts />
            </div>
          )}
          
          {activeTab === "explorer" && (
            <EnhancedDataExplorer />
          )}
          
          {activeTab === "settings" && (
            <SettingsPanel 
              refreshInterval={refreshInterval}
              setRefreshInterval={setRefreshInterval}
              autoRefresh={autoRefresh}
              setAutoRefresh={setAutoRefresh}
            />
          )}
        </motion.div>
      </AnimatePresence>
    </DashboardLayout>
  );
}
