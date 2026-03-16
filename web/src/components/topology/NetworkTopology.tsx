"use client";

import { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { 
  ReactFlow, 
  Node, 
  Edge, 
  useNodesState, 
  useEdgesState, 
  Controls, 
  MiniMap, 
  Background,
  MarkerType,
  applyNodeChanges,
  NodeChange,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Network, ArrowLeftRight, ArrowRight } from "lucide-react";
import { api } from "@/lib/api";
import { ClusterNode, ClusterNodeData } from "./ClusterNode";
import type { NodeTypes } from "@xyflow/react";

const nodeTypes: NodeTypes = {
  cluster: ClusterNode as any,
};

type LayoutType = "circle" | "grid" | "force";

export function NetworkTopology() {
  const [nodes, setNodes] = useNodesState<Node<ClusterNodeData>>([]);
  const [edges, setEdges] = useEdgesState<Edge>([]);
  const [loading, setLoading] = useState(true);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [layoutType, setLayoutType] = useState<LayoutType>("circle");
  
  // Track manually positioned nodes to preserve their positions across refreshes
  const manualPositions = useRef<Map<string, { x: number; y: number }>>(new Map());

  useEffect(() => {
    const fetchTopology = async () => {
      try {
        // Fetch initial gossip from seed
        const seedGossip = await api.getGossipMetrics();
        
        // Discover all nodes recursively by following peer references
        const discoveredNodes = new Set<string>([seedGossip.node_url]);
        const toFetch = new Set<string>([seedGossip.node_url]);
        const gossipData = new Map<string, any>();
        
        // Add seed's peers to discovery
        seedGossip.peer_details.forEach(p => {
          discoveredNodes.add(p.url);
          toFetch.add(p.url);
        });
        
        // Fetch from all discovered nodes (iteratively discover more)
        while (toFetch.size > 0) {
          const batch = Array.from(toFetch);
          toFetch.clear();
          
          const batchPromises = batch.map(async (nodeUrl) => {
            try {
              const gossip = await api.getGossipMetrics(nodeUrl);
              gossipData.set(nodeUrl, { gossip, success: true });
              
              // Discover any new peers mentioned by this node
              gossip.peer_details.forEach(p => {
                if (!discoveredNodes.has(p.url)) {
                  discoveredNodes.add(p.url);
                  toFetch.add(p.url);
                }
              });
            } catch (err) {
              gossipData.set(nodeUrl, { gossip: null, success: false });
            }
          });
          
          await Promise.all(batchPromises);
        }
        
        // Convert to the expected format
        const gossipResults = Array.from(discoveredNodes).map(nodeUrl => {
          const data = gossipData.get(nodeUrl);
          return {
            nodeUrl,
            gossip: data?.gossip || null,
            success: data?.success || false,
          };
        });
        
        // Sort nodes alphabetically by URL for consistent positioning
        gossipResults.sort((a, b) => a.nodeUrl.localeCompare(b.nodeUrl));

        // Build cross-referenced opinion map: what each node says about its peers
        const peerOpinions = new Map<string, Array<{ status: string; lag: number }>>();
        gossipData.forEach((data) => {
          if (!data.success || !data.gossip) return;
          data.gossip.peer_details.forEach((pd: any) => {
            if (!peerOpinions.has(pd.url)) peerOpinions.set(pd.url, []);
            peerOpinions.get(pd.url)!.push({ status: pd.status, lag: pd.lag });
          });
        });

        const statusRank = (s: string) => s === "dead" ? 2 : s === "stale" ? 1 : 0;
        const computeConsensus = (nodeUrl: string, isReachable: boolean): { status: "active" | "stale" | "dead"; lag: number } => {
          if (!isReachable) return { status: "dead", lag: 0 };
          const opinions = peerOpinions.get(nodeUrl) || [];
          if (opinions.length === 0) return { status: "active", lag: 0 };
          const worst = opinions.reduce((best, curr) =>
            statusRank(curr.status) > statusRank(best.status) ? curr : best,
            { status: "active", lag: 0 }
          );
          return worst as { status: "active" | "stale" | "dead"; lag: number };
        };
        
        // Build nodes and edges
        const newNodes: Node<ClusterNodeData>[] = [];
        const edgeMap = new Map<string, { from: string; to: string; bidirectional: boolean }>();
        
        gossipResults.forEach(({ nodeUrl, gossip, success }, index) => {
          // Calculate position based on layout type
          const totalNodes = gossipResults.length;
          let calculatedX = 0;
          let calculatedY = 0;
          
          if (layoutType === "circle") {
            // Circular layout
            const radius = 350;
            const centerX = 500;
            const centerY = 400;
            const angle = (index * 2 * Math.PI) / totalNodes - Math.PI / 2;
            calculatedX = centerX + radius * Math.cos(angle);
            calculatedY = centerY + radius * Math.sin(angle);
          } else if (layoutType === "grid") {
            // Grid layout
            const cols = Math.ceil(Math.sqrt(totalNodes));
            const row = Math.floor(index / cols);
            const col = index % cols;
            calculatedX = 150 + col * 300;
            calculatedY = 150 + row * 250;
          } else if (layoutType === "force") {
            // Honeycomb-like deterministic layout (no random — prevents shuffling on refresh)
            const cols = Math.ceil(Math.sqrt(totalNodes));
            const row = Math.floor(index / cols);
            const col = index % cols;
            const rowOffset = row % 2 === 0 ? 0 : 150;
            calculatedX = 150 + col * 300 + rowOffset;
            calculatedY = 200 + row * 220;
          }
          
          // Use manual position if available, otherwise use calculated position
          const manualPos = manualPositions.current.get(nodeUrl);
          const x = manualPos?.x ?? calculatedX;
          const y = manualPos?.y ?? calculatedY;
          
          if (success && gossip) {
            const consensus = computeConsensus(nodeUrl, true);
            // Create node
            const nodeData: ClusterNodeData = {
              url: nodeUrl,
              status: consensus.status,
              peers: gossip.peer_details.length,
              heartbeat: gossip.node_heartbeat,
              generation: gossip.generation || 0,
              lag: consensus.lag,
              isSeed: nodeUrl === seedGossip.node_url,
            };
            
            newNodes.push({
              id: nodeUrl,
              type: "cluster",
              position: { x, y },
              data: nodeData,
              className: selectedNodeId && selectedNodeId !== nodeUrl ? "opacity-30" : "",
            });
            
            // Create edges from this node to its peers
            gossip.peer_details.forEach((peer: any) => {
              const edgeKey = [nodeUrl, peer.url].sort().join("--");
              
              if (edgeMap.has(edgeKey)) {
                // Mark as bidirectional
                const existing = edgeMap.get(edgeKey)!;
                edgeMap.set(edgeKey, { ...existing, bidirectional: true });
              } else {
                // Add new edge
                edgeMap.set(edgeKey, {
                  from: nodeUrl,
                  to: peer.url,
                  bidirectional: false,
                });
              }
            });
          } else {
            // Dead node
            newNodes.push({
              id: nodeUrl,
              type: "cluster",
              position: { x, y },
              data: {
                url: nodeUrl,
                status: "dead",
                peers: 0,
                heartbeat: 0,
                generation: 0,
                lag: 0,
                isSeed: nodeUrl === seedGossip.node_url,
              },
              className: selectedNodeId && selectedNodeId !== nodeUrl ? "opacity-30" : "",
            });
          }
        });
        
        
        
        // Helper function to find the shortest connection between two nodes
        const findShortestHandles = (fromNode: Node, toNode: Node) => {
          const handles = [
            { name: "top", offset: { x: 0, y: -20 } },
            { name: "right", offset: { x: 120, y: 0 } },
            { name: "bottom", offset: { x: 0, y: 40 } },
            { name: "left", offset: { x: -120, y: 0 } },
          ];
          
          let shortestDistance = Infinity;
          let bestPair = { source: "bottom", target: "top" };
          
          // Try all combinations of source and target handles
          for (const sourceHandle of handles) {
            for (const targetHandle of handles) {
              const sourcePos = {
                x: fromNode.position.x + sourceHandle.offset.x,
                y: fromNode.position.y + sourceHandle.offset.y,
              };
              const targetPos = {
                x: toNode.position.x + targetHandle.offset.x,
                y: toNode.position.y + targetHandle.offset.y,
              };
              
              const distance = Math.sqrt(
                Math.pow(targetPos.x - sourcePos.x, 2) + 
                Math.pow(targetPos.y - sourcePos.y, 2)
              );
              
              if (distance < shortestDistance) {
                shortestDistance = distance;
                bestPair = { source: sourceHandle.name, target: targetHandle.name };
              }
            }
          }
          
          return bestPair;
        };
        
        // Build edges array with shortest path routing
        const newEdges: Edge[] = [];
        edgeMap.forEach(({ from, to, bidirectional }, key) => {
          const fromNode = newNodes.find(n => n.id === from);
          const toNode = newNodes.find(n => n.id === to);
          
          let sourceHandle = undefined;
          let targetHandle = undefined;
          
          if (fromNode && toNode) {
            const handles = findShortestHandles(fromNode, toNode);
            sourceHandle = handles.source;
            targetHandle = handles.target;
          }
          
          // Determine if this edge is connected to the selected node
          const isConnected = selectedNodeId ? (from === selectedNodeId || to === selectedNodeId) : false;
          const isDimmed = selectedNodeId && !isConnected;
          
          newEdges.push({
            id: key,
            source: from,
            target: to,
            sourceHandle,
            targetHandle,
            type: "default", // Curvy bezier lines
            animated: !bidirectional, // Always animate one-way connections
            style: {
              stroke: isConnected 
                ? (bidirectional ? "#84cc16" : "#eab308") 
                : (bidirectional ? "#84cc16" : "#eab308"),
              strokeWidth: isConnected 
                ? (bidirectional ? 4 : 3.5) 
                : (bidirectional ? 2.5 : 2),
              strokeDasharray: bidirectional ? "5,5" : undefined,
              opacity: isDimmed ? 0.15 : 1,
              filter: isConnected ? "drop-shadow(0 0 4px rgba(132, 204, 22, 0.6))" : undefined,
            },
            markerEnd: bidirectional ? undefined : {
              type: MarkerType.ArrowClosed,
              color: isConnected ? "#eab308" : "#eab308",
            },
          });
        });
        
        setNodes(newNodes);
        setEdges(newEdges);
        setLoading(false);
      } catch (err) {
        console.error("Failed to fetch topology", err);
        setLoading(false);
      }
    };

    fetchTopology();
    const interval = setInterval(fetchTopology, 5000);
    return () => clearInterval(interval);
  }, [setNodes, setEdges, selectedNodeId, layoutType]); // Re-run when selection or layout changes

  const handleLayoutChange = useCallback((newLayout: LayoutType) => {
    setLayoutType(newLayout);
    // Clear manual positions when switching layouts
    manualPositions.current.clear();
  }, []);

  const handleNodeClick = useCallback((_event: React.MouseEvent, node: Node) => {
    setSelectedNodeId(current => current === node.id ? null : node.id);
  }, []);

  const handlePaneClick = useCallback(() => {
    setSelectedNodeId(null);
  }, []);

  const edgeTypeCounts = useMemo(() => {
    const bidirectional = edges.filter(e => !e.animated).length; // Bidirectional are NOT animated
    const unidirectional = edges.length - bidirectional;
    return { bidirectional, unidirectional };
  }, [edges]);

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Network className="h-5 w-5" />
            Network Topology
          </CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center h-96">
          <div className="text-muted-foreground">Loading topology...</div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="h-full flex flex-col">
      <CardHeader className="border-b">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Network className="h-5 w-5" />
            Network Topology
          </CardTitle>
          <div className="flex items-center gap-4">
            {/* Layout Selector */}
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">Layout:</span>
              <div className="flex gap-1">
                <button
                  onClick={() => handleLayoutChange("circle")}
                  className={`px-3 py-1 text-xs rounded-md transition-colors ${
                    layoutType === "circle"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  Circle
                </button>
                <button
                  onClick={() => handleLayoutChange("grid")}
                  className={`px-3 py-1 text-xs rounded-md transition-colors ${
                    layoutType === "grid"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  Grid
                </button>
                <button
                  onClick={() => handleLayoutChange("force")}
                  className={`px-3 py-1 text-xs rounded-md transition-colors ${
                    layoutType === "force"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  Force
                </button>
              </div>
            </div>
            
            {/* Edge Stats */}
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="gap-1">
                <ArrowLeftRight className="h-3 w-3 text-green-600" />
                {edgeTypeCounts.bidirectional} Bidirectional
              </Badge>
              <Badge variant="outline" className="gap-1">
                <ArrowRight className="h-3 w-3 text-yellow-600" />
                {edgeTypeCounts.unidirectional} One-way
              </Badge>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex-1 p-0 relative">
        <div className="absolute inset-0">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={(changes: NodeChange[]) => {
              // Track manual position changes
              changes.forEach((change) => {
                if (change.type === 'position' && change.position && !change.dragging) {
                  // Position change completed (dragging finished)
                  manualPositions.current.set(change.id, change.position);
                }
              });
              setNodes((nds) => applyNodeChanges(changes, nds) as Node<ClusterNodeData>[]);
            }}
            fitView
            minZoom={0.5}
            maxZoom={2}
            nodesDraggable={true}
            nodesConnectable={false}
            elementsSelectable={true}
            onNodeClick={handleNodeClick}
            onPaneClick={handlePaneClick}
            defaultEdgeOptions={{
              animated: false,
            }}
          >
          <Background />
          <Controls className="!border !shadow-sm" />
          <MiniMap
            nodeColor={(node) => {
              const data = node.data as ClusterNodeData;
              return data.status === "active" ? "#84cc16" : data.status === "stale" ? "#eab308" : "#ef4444";
            }}
            className="!border !shadow-sm !bg-background"
          />
          </ReactFlow>
        </div>
        
        {/* Enhanced Legend */}
        <div className="absolute bottom-4 left-4 bg-background/95 backdrop-blur-sm border rounded-lg p-4 shadow-lg text-xs max-w-xs">
          <div className="font-semibold text-sm mb-3 flex items-center gap-2">
            <Network className="h-4 w-4" />
            Legend
          </div>
          
          {/* Connections Section */}
          <div className="space-y-2 mb-3 pb-3 border-b">
            <div className="text-muted-foreground font-medium mb-1.5">Connections</div>
            <div className="flex items-center gap-2">
              <div className="relative w-10 h-0.5 bg-green-500">
                <div className="absolute inset-0 bg-green-500 opacity-50 animate-pulse"></div>
              </div>
              <span className="flex-1">Bidirectional gossip</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="relative w-10">
                <svg width="40" height="8" viewBox="0 0 40 8">
                  <defs>
                    <marker id="arrow" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto" markerUnits="strokeWidth">
                      <path d="M0,0 L0,6 L9,3 z" fill="#eab308" />
                    </marker>
                  </defs>
                  <line x1="0" y1="4" x2="36" y2="4" stroke="#eab308" strokeWidth="2" markerEnd="url(#arrow)">
                    <animate attributeName="stroke-dashoffset" from="0" to="100" dur="2s" repeatCount="indefinite" />
                  </line>
                </svg>
              </div>
              <span className="flex-1">One-way gossip</span>
            </div>
            <div className="text-[10px] text-muted-foreground italic mt-1 pl-12">
              Click a node to highlight its connections
            </div>
          </div>
          
          {/* Node Status Section */}
          <div className="space-y-2">
            <div className="text-muted-foreground font-medium mb-1.5">Node Status</div>
            <div className="flex items-center gap-2">
              <div className="relative w-3 h-3">
                <div className="absolute inset-0 bg-green-500 rounded-full animate-ping opacity-75"></div>
                <div className="relative w-3 h-3 bg-green-500 rounded-full"></div>
              </div>
              <span className="flex-1">Active</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 bg-yellow-500 rounded-full"></div>
              <span className="flex-1">Stale</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-3 h-3 bg-red-500 rounded-full"></div>
              <span className="flex-1">Dead</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
