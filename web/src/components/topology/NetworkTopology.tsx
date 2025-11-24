"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import {
  ReactFlow,
  Node,
  Edge,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  MarkerType,
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

export function NetworkTopology() {
  const [nodes, setNodes] = useNodesState<Node<ClusterNodeData>>([]);
  const [edges, setEdges] = useEdgesState<Edge>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchTopology = async () => {
      try {
        // Fetch initial gossip from seed
        const seedGossip = await api.getGossipMetrics();
        
        // Build list of all nodes
        const allNodeUrls = [seedGossip.node_url, ...seedGossip.peer_details.map(p => p.url)];
        
        // Fetch gossip from each node
        const gossipPromises = allNodeUrls.map(async (nodeUrl) => {
          try {
            const gossip = await api.getGossipMetrics(nodeUrl);
            return { nodeUrl, gossip, success: true };
          } catch (err) {
            return { nodeUrl, gossip: null, success: false };
          }
        });
        
        const gossipResults = await Promise.all(gossipPromises);
        
        // Sort nodes alphabetically by URL for consistent positioning
        gossipResults.sort((a, b) => a.nodeUrl.localeCompare(b.nodeUrl));
        
        // Build nodes and edges
        const newNodes: Node<ClusterNodeData>[] = [];
        const edgeMap = new Map<string, { from: string; to: string; bidirectional: boolean }>();
        
        gossipResults.forEach(({ nodeUrl, gossip, success }, index) => {
          // Calculate circular position
          const totalNodes = gossipResults.length;
          const radius = 250; // Distance from center
          const centerX = 400;
          const centerY = 300;
          const angle = (index * 2 * Math.PI) / totalNodes - Math.PI / 2; // Start from top
          const x = centerX + radius * Math.cos(angle);
          const y = centerY + radius * Math.sin(angle);
          
          if (success && gossip) {
            // Create node
            const nodeData: ClusterNodeData = {
              url: nodeUrl,
              status: "active",
              peers: gossip.peer_details.length,
              heartbeat: gossip.node_heartbeat,
              lag: 0,
              isSeed: nodeUrl === seedGossip.node_url,
            };
            
            newNodes.push({
              id: nodeUrl,
              type: "cluster",
              position: { x, y },
              data: nodeData,
            });
            
            // Create edges from this node to its peers
            gossip.peer_details.forEach((peer) => {
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
                lag: 0,
                isSeed: nodeUrl === seedGossip.node_url,
              },
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
          
          newEdges.push({
            id: key,
            source: from,
            target: to,
            sourceHandle,
            targetHandle,
            type: "straight", // Use straight lines for shortest path
            animated: bidirectional,
            style: {
              stroke: bidirectional ? "#84cc16" : "#eab308",
              strokeWidth: bidirectional ? 2.5 : 2,
            },
            markerEnd: bidirectional ? undefined : {
              type: MarkerType.ArrowClosed,
              color: "#eab308",
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
  }, [setNodes, setEdges]);

  const edgeTypeCounts = useMemo(() => {
    const bidirectional = edges.filter(e => e.animated).length;
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
          <div className="flex items-center gap-3">
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
            fitView
            minZoom={0.5}
            maxZoom={2}
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
        
        {/* Legend */}
        <div className="absolute bottom-4 left-4 bg-background/95 backdrop-blur-sm border rounded-lg p-3 shadow-lg text-xs space-y-2">
          <div className="font-semibold mb-2">Legend</div>
          <div className="flex items-center gap-2">
            <div className="w-8 h-0.5 bg-green-500"></div>
            <span>Bidirectional (mutual peers)</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-8 h-0.5 bg-yellow-500"></div>
            <ArrowRight className="h-3 w-3 text-yellow-600" />
            <span>One-way (asymmetric)</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-3 h-3 bg-green-500 rounded-full"></div>
            <span>Active node</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-3 h-3 bg-red-500 rounded-full"></div>
            <span>Dead node</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
