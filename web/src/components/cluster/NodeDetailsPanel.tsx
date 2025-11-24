"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { GossipMetrics, NodeStatus } from "@/lib/types";
import { Server, Activity, Database, Code, Heart, Clock } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";

interface NodeDetailsPanelProps {
  nodes: Record<string, NodeStatus>;
  onNodeChange?: (nodeUrl: string | null) => void;
}

export function NodeDetailsPanel({ nodes, onNodeChange }: NodeDetailsPanelProps) {
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [nodeGossip, setNodeGossip] = useState<GossipMetrics | null>(null);
  const [nodeKeys, setNodeKeys] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [lastHeartbeat, setLastHeartbeat] = useState<number>(0);

  const nodesList = Object.keys(nodes).sort((a, b) => a.localeCompare(b));

  // Fetch detailed data for selected node
  useEffect(() => {
    if (!selectedNode) {
      setNodeGossip(null);
      setNodeKeys(null);
      return;
    }

    const fetchNodeDetails = async () => {
      setLoading(true);
      try {
        // Fetch gossip metrics from the selected node
        const gossipRes = await fetch(`/api/proxy/cluster/gossip?node=${encodeURIComponent(selectedNode)}`);
        const gossipData = await gossipRes.json();
        setNodeGossip(gossipData);
        
        // Track heartbeat changes for animation
        if (gossipData.node_heartbeat !== lastHeartbeat) {
          setLastHeartbeat(gossipData.node_heartbeat);
        }

        // Fetch keys from the selected node
        const keysRes = await fetch(`/api/proxy/keys?node=${encodeURIComponent(selectedNode)}&pageSize=10`);
        const keysData = await keysRes.json();
        setNodeKeys(keysData);
      } catch (error) {
        console.error("Failed to fetch node details:", error);
      } finally {
        setLoading(false);
      }
    };

    fetchNodeDetails();
    
    // Auto-refresh every 2 seconds
    const interval = setInterval(fetchNodeDetails, 2000);
    return () => clearInterval(interval);
  }, [selectedNode]);

  const handleNodeSelect = (value: string) => {
    setSelectedNode(value);
    onNodeChange?.(value);
  };

  const selectedNodeData = selectedNode ? nodes[selectedNode] : null;

  return (
    <Card className="w-full">
      <CardHeader>
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div className="flex-1">
            <CardTitle className="flex items-center gap-2 mb-2">
              <Server className="h-5 w-5" />
              Node Inspector
            </CardTitle>
            <CardDescription>
              Select any node to inspect its gossip map, metrics, and stored keys in real-time
            </CardDescription>
          </div>
          <Select value={selectedNode || ""} onValueChange={handleNodeSelect}>
            <SelectTrigger className="w-full sm:w-[350px]">
              <SelectValue placeholder="Choose a node to inspect..." />
            </SelectTrigger>
            <SelectContent>
              <div className="px-2 py-1.5 text-xs text-muted-foreground font-semibold">
                All nodes are equal - no primary/leader
              </div>
              {nodesList.map((nodeUrl) => {
                const status = nodes[nodeUrl]?.status;
                return (
                  <SelectItem key={nodeUrl} value={nodeUrl}>
                    <div className="flex items-center gap-2">
                      <div className={`h-2 w-2 rounded-full ${
                        status === 'active' ? 'bg-green-500' : 
                        status === 'stale' ? 'bg-yellow-500' : 'bg-red-500'
                      }`} />
                      <span className="font-mono text-sm">{nodeUrl}</span>
                    </div>
                  </SelectItem>
                );
              })}
            </SelectContent>
          </Select>
        </div>
      </CardHeader>
      <CardContent>
        {!selectedNode ? (
          <div className="flex flex-col items-center justify-center h-96 text-center space-y-4">
            <div className="relative">
              <Server className="h-20 w-20 opacity-10" />
              <Activity className="h-10 w-10 absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-muted-foreground animate-pulse" />
            </div>
            <div className="space-y-2">
              <p className="text-lg font-medium">No Node Selected</p>
              <p className="text-sm text-muted-foreground max-w-md">
                Use the dropdown above to select any node and view its complete state: 
                gossip protocol details, real-time metrics, and stored keys
              </p>
              <p className="text-xs text-muted-foreground pt-2 border-t">
                💡 All {nodesList.length} nodes are treated equally - there&apos;s no concept of primary or leader
              </p>
            </div>
          </div>
        ) : (
          <AnimatePresence mode="wait">
            <motion.div
              key={selectedNode}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              transition={{ duration: 0.2 }}
            >
              {/* Node Summary */}
              <div className="mb-6 p-4 bg-muted/50 rounded-lg">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-lg font-semibold">{selectedNode}</h3>
                  {selectedNodeData && (
                    <Badge 
                      variant={selectedNodeData.status === "active" ? "default" : "destructive"}
                      className={selectedNodeData.status === "active" ? "bg-green-500" : ""}
                    >
                      {selectedNodeData.status}
                    </Badge>
                  )}
                </div>
                {nodeGossip && (
                  <div className="grid grid-cols-4 gap-4 text-sm">
                    <div>
                      <p className="text-muted-foreground">Heartbeat</p>
                      <motion.p 
                        key={nodeGossip.node_heartbeat}
                        initial={{ scale: 1.2, color: "#84cc16" }}
                        animate={{ scale: 1, color: "inherit" }}
                        className="font-mono font-bold"
                      >
                        {nodeGossip.node_heartbeat}
                      </motion.p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">Health</p>
                      <p className="font-semibold capitalize">{nodeGossip.cluster_health}</p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">Active Peers</p>
                      <p className="font-mono">{nodeGossip.active_peers}/{nodeGossip.total_peers}</p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">Convergence</p>
                      <p className="font-mono">{nodeGossip.convergence_rate.toFixed(1)}%</p>
                    </div>
                  </div>
                )}
              </div>

              <Tabs defaultValue="gossip" className="w-full">
                <TabsList className="grid w-full grid-cols-4">
                  <TabsTrigger value="gossip" className="gap-2">
                    <Activity className="h-4 w-4" />
                    Gossip Map
                  </TabsTrigger>
                  <TabsTrigger value="metrics" className="gap-2">
                    <Heart className="h-4 w-4" />
                    Metrics
                  </TabsTrigger>
                  <TabsTrigger value="keys" className="gap-2">
                    <Database className="h-4 w-4" />
                    Keys
                  </TabsTrigger>
                  <TabsTrigger value="raw" className="gap-2">
                    <Code className="h-4 w-4" />
                    Raw JSON
                  </TabsTrigger>
                </TabsList>

                {/* Gossip Map Tab */}
                <TabsContent value="gossip" className="mt-4">
                  {nodeGossip?.peer_details ? (
                    <div className="rounded-md border">
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Peer URL</TableHead>
                            <TableHead>Status</TableHead>
                            <TableHead className="text-right">Heartbeat</TableHead>
                            <TableHead className="text-right">Lag</TableHead>
                            <TableHead className="text-right">Delta</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {[...nodeGossip.peer_details]
                            .sort((a, b) => a.url.localeCompare(b.url))
                            .map((peer, idx) => {
                              const heartbeatDelta = nodeGossip.node_heartbeat - peer.heartbeat;
                              return (
                                <TableRow key={idx}>
                                  <TableCell className="font-mono text-sm">{peer.url}</TableCell>
                                  <TableCell>
                                    <Badge
                                      variant={
                                        peer.status === "active"
                                          ? "default"
                                          : peer.status === "stale"
                                          ? "secondary"
                                          : "destructive"
                                      }
                                      className={
                                        peer.status === "active" ? "bg-green-500" : 
                                        peer.status === "stale" ? "bg-yellow-500" : ""
                                      }
                                    >
                                      {peer.status}
                                    </Badge>
                                  </TableCell>
                                  <TableCell className="text-right font-mono">{peer.heartbeat}</TableCell>
                                  <TableCell className="text-right">
                                    <span
                                      className={
                                        peer.lag === 0
                                          ? "text-green-500"
                                          : peer.lag <= 5
                                          ? "text-yellow-500"
                                          : "text-red-500"
                                      }
                                    >
                                      {peer.lag}
                                    </span>
                                  </TableCell>
                                  <TableCell className="text-right font-mono text-muted-foreground">
                                    {heartbeatDelta >= 0 ? "+" : ""}{heartbeatDelta}
                                  </TableCell>
                                </TableRow>
                              );
                            })}
                        </TableBody>
                      </Table>
                    </div>
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      {loading ? "Loading gossip data..." : "No gossip data available"}
                    </div>
                  )}
                </TabsContent>

                {/* Metrics Tab */}
                <TabsContent value="metrics" className="mt-4">
                  {nodeGossip ? (
                    <div className="grid grid-cols-2 gap-4">
                      <Card>
                        <CardHeader className="pb-3">
                          <CardTitle className="text-sm font-medium">Round-Trip Updates</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="text-2xl font-bold">~10s</div>
                          <p className="text-xs text-muted-foreground">Gossip interval</p>
                        </CardContent>
                      </Card>
                      <Card>
                        <CardHeader className="pb-3">
                          <CardTitle className="text-sm font-medium">Known Peers</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="text-2xl font-bold">{nodeGossip.total_peers}</div>
                          <p className="text-xs text-muted-foreground">Total discovered</p>
                        </CardContent>
                      </Card>
                      <Card>
                        <CardHeader className="pb-3">
                          <CardTitle className="text-sm font-medium">Average Lag</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="text-2xl font-bold">{nodeGossip.average_lag.toFixed(2)}</div>
                          <p className="text-xs text-muted-foreground">Heartbeat difference</p>
                        </CardContent>
                      </Card>
                      <Card>
                        <CardHeader className="pb-3">
                          <CardTitle className="text-sm font-medium">Max Lag</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="text-2xl font-bold">{nodeGossip.max_lag}</div>
                          <p className="text-xs text-muted-foreground">Worst peer sync</p>
                        </CardContent>
                      </Card>
                      <Card>
                        <CardHeader className="pb-3">
                          <CardTitle className="text-sm font-medium">Last Sync</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="text-2xl font-bold flex items-center gap-2">
                            <Clock className="h-6 w-6" />
                            <span>Live</span>
                          </div>
                          <p className="text-xs text-muted-foreground">Auto-refresh: 2s</p>
                        </CardContent>
                      </Card>
                      <Card>
                        <CardHeader className="pb-3">
                          <CardTitle className="text-sm font-medium">Peer Heartbeat Delta</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="text-2xl font-bold">
                            {nodeGossip.peer_details.reduce((sum, p) => sum + Math.abs(nodeGossip.node_heartbeat - p.heartbeat), 0)}
                          </div>
                          <p className="text-xs text-muted-foreground">Total deviation</p>
                        </CardContent>
                      </Card>
                    </div>
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      {loading ? "Loading metrics..." : "No metrics available"}
                    </div>
                  )}
                </TabsContent>

                {/* Keys Tab */}
                <TabsContent value="keys" className="mt-4">
                  {nodeKeys ? (
                    <div>
                      <div className="mb-3 text-sm text-muted-foreground">
                        Showing {nodeKeys.keys?.length || 0} of {nodeKeys.total || 0} keys stored on this node
                      </div>
                      <div className="rounded-md border">
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead>Key</TableHead>
                              <TableHead>Value</TableHead>
                              <TableHead className="text-right">Size</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {nodeKeys.keys && nodeKeys.keys.length > 0 ? (
                              nodeKeys.keys.map((item: any, idx: number) => (
                                <TableRow key={idx}>
                                  <TableCell className="font-mono text-sm">{item.key}</TableCell>
                                  <TableCell className="font-mono text-sm max-w-md truncate">
                                    {item.value}
                                  </TableCell>
                                  <TableCell className="text-right text-muted-foreground">
                                    {item.size} bytes
                                  </TableCell>
                                </TableRow>
                              ))
                            ) : (
                              <TableRow>
                                <TableCell colSpan={3} className="text-center text-muted-foreground">
                                  No keys stored on this node
                                </TableCell>
                              </TableRow>
                            )}
                          </TableBody>
                        </Table>
                      </div>
                    </div>
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      {loading ? "Loading keys..." : "No keys data available"}
                    </div>
                  )}
                </TabsContent>

                {/* Raw JSON Tab */}
                <TabsContent value="raw" className="mt-4">
                  {nodeGossip ? (
                    <pre className="bg-muted p-4 rounded-md overflow-x-auto text-xs">
                      {JSON.stringify({ gossip: nodeGossip, keys: nodeKeys }, null, 2)}
                    </pre>
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      {loading ? "Loading data..." : "No data available"}
                    </div>
                  )}
                </TabsContent>
              </Tabs>
            </motion.div>
          </AnimatePresence>
        )}
      </CardContent>
    </Card>
  );
}
