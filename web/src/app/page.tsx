"use client";

import { useEffect, useState, useRef } from "react";
import Image from "next/image";
import { SummaryCards } from "@/components/dashboard/SummaryCards";
import { ClusterStatusTable } from "@/components/dashboard/ClusterStatusTable";
import { RingVisualizer } from "@/components/dashboard/RingVisualizer";
import { EnhancedDataExplorer } from "@/components/dashboard/EnhancedDataExplorer";
import { PerformanceCharts } from "@/components/metrics/PerformanceCharts";
import { RingDistributionChart } from "@/components/metrics/RingDistributionChart";
import { GossipViewer } from "@/components/gossip/GossipViewer";
import { EventLog } from "@/components/events/EventLog";
import { SettingsPanel } from "@/components/settings/SettingsPanel";
import { CommandPalette } from "@/components/command-palette";
import { ThemeToggle } from "@/components/theme-toggle";
import { NetworkTopology } from "@/components/topology/NetworkTopology";
import { api } from "@/lib/api";
import { NodeStatus, RingState, GossipMetrics } from "@/lib/types";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RefreshCw, Gauge, Activity, Network, Clock, Settings as SettingsIcon, Command } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { motion, AnimatePresence } from "framer-motion";
import { Badge } from "@/components/ui/badge";

export default function Dashboard() {
  const [nodes, setNodes] = useState<Record<string, NodeStatus>>({});
  const [selfNode, setSelfNode] = useState<string | null>(null);
  const [ring, setRing] = useState<RingState>({ ranges: {} });
  const [gossip, setGossip] = useState<GossipMetrics | null>(null);
  const [discoveredNodes, setDiscoveredNodes] = useState<string[]>([]);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState("overview");
  const [connectionStatus, setConnectionStatus] = useState<"connected" | "disconnected" | "connecting">("connecting");
  
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
      
      // Use gossip as source of truth for cluster status
      const [clusterData, ringData] = await Promise.all([
        api.getClusterStatus(), // Returns { gossip, nodes, selfNode }
        api.getRingState(),
      ]);
      
      setNodes(clusterData.nodes);
      setSelfNode(clusterData.selfNode);
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
      if (e.key >= "1" && e.key <= "5" && !e.metaKey && !e.ctrlKey) {
        const tabs = ["overview", "metrics", "explorer", "events", "settings"];
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

  return (
    <div className="min-h-screen bg-gradient-to-br from-background via-background to-muted/20">
      <CommandPalette onNavigate={setActiveTab} onRefresh={fetchData} />
      
      {/* Glassmorphism Header */}
      <div className="sticky top-0 z-50 border-b backdrop-blur-xl bg-background/80">
        <div className="container mx-auto px-8 py-4">
          <div className="flex items-center justify-between">
            <motion.div
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.5 }}
              className="flex items-center gap-4"
            >
              <Image 
                src="/logo.png" 
                alt="LimeDB Logo" 
                width={180} 
                height={40}
                className="h-10 w-auto"
                priority
              />
              <div>
                <p className="text-sm text-muted-foreground flex items-center gap-2">
                  
                  <Badge variant={connectionStatus === "connected" ? "default" : "destructive"} className="text-xs">
                    {connectionStatus}
                  </Badge>
                </p>
              </div>
            </motion.div>
            <div className="flex items-center gap-3">
              <motion.div
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                className="text-sm text-muted-foreground flex items-center gap-2"
              >
                <Clock className="h-4 w-4" />
                {lastUpdated.toLocaleTimeString()}
              </motion.div>
              <Button
                variant="outline"
                size="sm"
                onClick={fetchData}
                disabled={loading}
                className="gap-2"
              >
                <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
                Refresh
              </Button>
              <ThemeToggle />
              <Button variant="ghost" size="sm" className="gap-2 hidden md:flex">
                <Command className="h-4 w-4" />
                <kbd className="pointer-events-none inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground opacity-100">
                  <span className="text-xs">⌘</span>K
                </kbd>
              </Button>
            </div>
          </div>
        </div>
      </div>

      <div className="container mx-auto p-8 space-y-6">
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

        {/* Summary with animation */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.1 }}
        >
          <SummaryCards
            health={health}
            activeNodes={activeNodes}
            totalKeys={totalKeys}
          />
        </motion.div>

        {/* Main Content with enhanced tabs */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.2 }}
        >
          <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
            <TabsList className="grid w-full grid-cols-5 lg:w-[700px] bg-muted/50 backdrop-blur-sm">
              <TabsTrigger value="overview" className="gap-2 data-[state=active]:bg-background">
                <Gauge className="h-4 w-4" />
                <span className="hidden sm:inline">Overview</span>
              </TabsTrigger>
              <TabsTrigger value="metrics" className="gap-2 data-[state=active]:bg-background">
                <Activity className="h-4 w-4" />
                <span className="hidden sm:inline">Metrics</span>
              </TabsTrigger>
              <TabsTrigger value="explorer" className="gap-2 data-[state=active]:bg-background">
                <Network className="h-4 w-4" />
                <span className="hidden sm:inline">Explorer</span>
              </TabsTrigger>
              <TabsTrigger value="events" className="gap-2 data-[state=active]:bg-background">
                <Clock className="h-4 w-4" />
                <span className="hidden sm:inline">Events</span>
              </TabsTrigger>
              <TabsTrigger value="settings" className="gap-2 data-[state=active]:bg-background">
                <SettingsIcon className="h-4 w-4" />
                <span className="hidden sm:inline">Settings</span>
              </TabsTrigger>
            </TabsList>
            
            <AnimatePresence mode="wait">
              <motion.div
                key={activeTab}
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.3 }}
              >
                <TabsContent value="overview" className="space-y-4 mt-0">
                  <div className="grid gap-4 lg:grid-cols-2">
                    <ClusterStatusTable nodes={nodes} />
                    <RingVisualizer ring={ring} />
                  </div>
                  <NetworkTopology nodes={nodes} />
                  <GossipViewer gossip={gossip} />
                </TabsContent>
                
                <TabsContent value="metrics" className="space-y-4 mt-0">
                  <PerformanceCharts />
                  <RingDistributionChart ring={ring} gossip={gossip} />
                </TabsContent>
                
                <TabsContent value="explorer" className="mt-0">
                  <EnhancedDataExplorer />
                </TabsContent>
                
                <TabsContent value="events" className="mt-0">
                  <EventLog />
                </TabsContent>
                
                <TabsContent value="settings" className="mt-0">
                  <SettingsPanel 
                    refreshInterval={refreshInterval}
                    setRefreshInterval={setRefreshInterval}
                    autoRefresh={autoRefresh}
                    setAutoRefresh={setAutoRefresh}
                  />
                </TabsContent>
              </motion.div>
            </AnimatePresence>
          </Tabs>
        </motion.div>
      </div>
    </div>
  );
}
