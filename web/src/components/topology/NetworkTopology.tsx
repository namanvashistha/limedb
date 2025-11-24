"use client";

import { useEffect, useRef, useState, useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { NodeStatus } from "@/lib/types";
import { Network } from "lucide-react";
import dynamic from "next/dynamic";

const ForceGraph2D = dynamic(() => import("react-force-graph-2d"), { ssr: false });

interface NetworkTopologyProps {
  nodes: Record<string, NodeStatus>;
}

interface GraphNode {
  id: string;
  name: string;
  val: number;
  color: string;
  status: string;
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
}

interface GraphLink {
  source: string;
  target: string;
  color: string;
}

export function NetworkTopology({ nodes }: NetworkTopologyProps) {
  const [graphData, setGraphData] = useState<{ nodes: GraphNode[]; links: GraphLink[] }>({
    nodes: [],
    links: [],
  });
  const [dimensions, setDimensions] = useState({ width: 800, height: 500 });
  const containerRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<any>(null);
  const prevNodesRef = useRef<Record<string, NodeStatus>>({});

  // Check if nodes have actually changed (deep comparison of keys and status)
  const nodesChanged = useMemo(() => {
    const currentKeys = Object.keys(nodes).sort().join(',');
    const prevKeys = Object.keys(prevNodesRef.current).sort().join(',');
    
    if (currentKeys !== prevKeys) return true;
    
    // Check if any node status changed
    for (const key in nodes) {
      if (nodes[key].status !== prevNodesRef.current[key]?.status) {
        return true;
      }
    }
    return false;
  }, [nodes]);

  useEffect(() => {
    // Only update if nodes actually changed
    if (!nodesChanged && graphData.nodes.length > 0) {
      return;
    }

    const nodesList = Object.values(nodes);
    if (nodesList.length === 0) {
      setGraphData({ nodes: [], links: [] });
      prevNodesRef.current = {};
      return;
    }

    // Preserve existing node positions
    const existingNodesMap = new Map(graphData.nodes.map(n => [n.id, n]));
    
    const graphNodes: GraphNode[] = [];
    const graphLinks: GraphLink[] = [];
    const processedLinks = new Set<string>();

    // Add all nodes, preserving positions if they exist
    nodesList.forEach((node) => {
      const existingNode = existingNodesMap.get(node.nodeUrl);
      graphNodes.push({
        id: node.nodeUrl,
        name: node.nodeUrl,
        val: node.status === "active" ? 30 : 20,
        color: node.status === "active" ? "#84cc16" : node.status === "stale" ? "#eab308" : "#ef4444",
        status: node.status,
        // Preserve position and velocity if node already exists
        ...(existingNode && {
          x: existingNode.x,
          y: existingNode.y,
          vx: existingNode.vx,
          vy: existingNode.vy,
        }),
      });

      // Add links to peers (avoid duplicates)
      if (node.peers && node.peers.length > 0) {
        node.peers.forEach((peer) => {
          const linkId = [node.nodeUrl, peer].sort().join("--");
          if (!processedLinks.has(linkId)) {
            processedLinks.add(linkId);
            graphLinks.push({
              source: node.nodeUrl,
              target: peer,
              color: node.status === "active" ? "#84cc16" : "#666",
            });
          }
        });
      }
    });

    setGraphData({ nodes: graphNodes, links: graphLinks });
    prevNodesRef.current = nodes;
  }, [nodes, nodesChanged]);

  // Configure forces for better separation (only once on mount)
  useEffect(() => {
    if (fgRef.current && graphData.nodes.length > 0) {
      const fg = fgRef.current;
      
      // Increase repulsion force
      fg.d3Force('charge')?.strength(-500);
      
      // Increase link distance
      fg.d3Force('link')?.distance(150);
      
      // Reduce center gravity
      fg.d3Force('center')?.strength(0.05);
    }
  }, [graphData.nodes.length]);

  useEffect(() => {
    const updateDimensions = () => {
      if (containerRef.current) {
        const width = containerRef.current.offsetWidth;
        setDimensions({ width, height: 500 });
      }
    };

    updateDimensions();
    window.addEventListener("resize", updateDimensions);
    return () => window.removeEventListener("resize", updateDimensions);
  }, []);

  if (graphData.nodes.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Network className="h-5 w-5" />
            Network Topology
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center justify-center h-64 text-muted-foreground space-y-2">
          <Network className="h-12 w-12 opacity-20" />
          <p>No topology data available</p>
          <p className="text-xs">Nodes will appear here once discovered</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Network className="h-5 w-5" />
            Network Topology
          </CardTitle>
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <div className="flex items-center gap-1">
              <div className="w-3 h-3 rounded-full bg-lime-500" />
              <span>Active</span>
            </div>
            <div className="flex items-center gap-1">
              <div className="w-3 h-3 rounded-full bg-yellow-500" />
              <span>Stale</span>
            </div>
            <div className="flex items-center gap-1">
              <div className="w-3 h-3 rounded-full bg-red-500" />
              <span>Dead</span>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div ref={containerRef} className="relative bg-muted/30 rounded-lg overflow-hidden">
          <ForceGraph2D
            ref={fgRef}
            graphData={graphData}
            width={dimensions.width}
            height={dimensions.height}
            nodeLabel="name"
            nodeColor="color"
            nodeRelSize={8}
            nodeVal="val"
            linkColor="color"
            linkWidth={2}
            backgroundColor="rgba(0,0,0,0)"
            linkDirectionalParticles={2}
            linkDirectionalParticleWidth={2}
            linkDirectionalParticleSpeed={0.005}
            enableNodeDrag={true}
            enableZoomInteraction={true}
            enablePanInteraction={true}
            cooldownTicks={100}
            d3AlphaDecay={0.02}
            d3VelocityDecay={0.3}
            onNodeClick={(node: any) => {
              // Node clicked - could add details modal here
            }}
            nodeCanvasObjectMode={() => "after"}
            nodeCanvasObject={(node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
              const label = node.name;
              const fontSize = 12 / globalScale;
              ctx.font = `${fontSize}px Sans-Serif`;
              ctx.textAlign = "center";
              ctx.textBaseline = "middle";
              ctx.fillStyle = "#ffffff";
              ctx.fillText(label.split(":").pop() || "", node.x, node.y + 20 / globalScale);
            }}
          />
          <div className="absolute bottom-4 right-4 text-xs text-muted-foreground bg-background/80 backdrop-blur-sm px-3 py-2 rounded-lg border">
            {graphData.nodes.length} node{graphData.nodes.length !== 1 ? "s" : ""} •{" "}
            {graphData.links.length} connection{graphData.links.length !== 1 ? "s" : ""}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
