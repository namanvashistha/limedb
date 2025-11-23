"use client";

import { useEffect, useRef, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { NodeStatus } from "@/lib/types";
import { Network, Maximize2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import dynamic from "next/dynamic";

const ForceGraph2D = dynamic(() => import("react-force-graph-2d"), { ssr: false });

interface NetworkTopologyProps {
  node: NodeStatus | null;
}

interface GraphNode {
  id: string;
  name: string;
  val: number;
  color: string;
  status: string;
}

interface GraphLink {
  source: string;
  target: string;
  color: string;
}

export function NetworkTopology({ node }: NetworkTopologyProps) {
  const [graphData, setGraphData] = useState<{ nodes: GraphNode[]; links: GraphLink[] }>({
    nodes: [],
    links: [],
  });
  const [dimensions, setDimensions] = useState({ width: 800, height: 500 });
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!node) {
      setGraphData({ nodes: [], links: [] });
      return;
    }

    const nodes: GraphNode[] = [];
    const links: GraphLink[] = [];

    // Add current node
    const currentNodeId = node.nodeUrl;
    nodes.push({
      id: currentNodeId,
      name: currentNodeId,
      val: 30,
      color: node.status === "active" ? "#84cc16" : "#ef4444",
      status: node.status,
    });

    // Add peer nodes
    if (node.peers && node.peers.length > 0) {
      node.peers.forEach((peer) => {
        nodes.push({
          id: peer,
          name: peer,
          val: 20,
          color: "#3b82f6",
          status: "peer",
        });

        // Create bidirectional link
        links.push({
          source: currentNodeId,
          target: peer,
          color: "#84cc16",
        });
      });
    }

    setGraphData({ nodes, links });
  }, [node]);

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

  if (!node || graphData.nodes.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Network className="h-5 w-5" />
            Network Topology
          </CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center h-64 text-muted-foreground">
          No cluster topology data available
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
              <span>Active Node</span>
            </div>
            <div className="flex items-center gap-1">
              <div className="w-3 h-3 rounded-full bg-blue-500" />
              <span>Peer Nodes</span>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div ref={containerRef} className="relative bg-muted/30 rounded-lg overflow-hidden">
          <ForceGraph2D
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
            onNodeClick={(node: any) => {
              console.log("Node clicked:", node);
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
