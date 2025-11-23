"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from "recharts";
import { RingState } from "@/lib/types";
import { GossipMetrics } from "@/lib/types";
import { Disc } from "lucide-react";

interface RingDistributionChartProps {
  ring: RingState;
  gossip: GossipMetrics | null;
}

const COLORS = ["#84cc16", "#3b82f6", "#a855f7", "#06b6d4", "#eab308", "#ef4444"];

export function RingDistributionChart({ ring, gossip }: RingDistributionChartProps) {
  // First, try to show key distribution if we have it
  const ranges = ring.ranges || {};
  const rangeNodes = Object.keys(ranges);
  
  const keyDistribution = rangeNodes.map((node, index) => {
    let nodeSize = 0;
    if (Array.isArray(ranges[node])) {
      ranges[node].forEach((r) => {
        nodeSize += r.size || 0;
      });
    }
    return {
      name: node,
      value: nodeSize,
      color: COLORS[index % COLORS.length],
    };
  }).filter(d => d.value > 0);

  // If we have key distribution, show it
  if (keyDistribution.length > 0) {
    return renderChart(keyDistribution, "Key Distribution");
  }

  // Otherwise, show token distribution based on gossip cluster nodes
  if (!gossip || !gossip.peer_details || gossip.peer_details.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Disc className="h-5 w-5" />
            Token Ring Distribution
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center justify-center h-64 text-muted-foreground space-y-2">
          <Disc className="h-12 w-12 opacity-20" />
          <p>No cluster nodes discovered</p>
          <p className="text-xs">Waiting for gossip discovery...</p>
        </CardContent>
      </Card>
    );
  }

  // Calculate token distribution based on cluster nodes
  // Each node gets an equal share of the ring (360 degrees / number of nodes)
  const totalNodes = gossip.peer_details.length;
  const virtualNodesPerNode = ring.virtualNodesPerNode || 2;
  
  const tokenDistribution = gossip.peer_details.map((peer, index) => ({
    name: peer.url,
    value: virtualNodesPerNode, // Each node has virtual nodes
    color: COLORS[index % COLORS.length],
    status: peer.status,
  }));

  return renderChart(tokenDistribution, "Token Distribution", totalNodes, virtualNodesPerNode);
}

function renderChart(
  chartData: Array<{ name: string; value: number; color: string; status?: string }>,
  title: string,
  totalNodes?: number,
  virtualNodesPerNode?: number
) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Disc className="h-5 w-5" />
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={300}>
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              innerRadius={60}
              outerRadius={100}
              fill="#8884d8"
              dataKey="value"
              label={({ name, percent }) => `${(name || '').split(":").pop()}: ${((percent ?? 0) * 100).toFixed(0)}%`}
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{
                backgroundColor: "#1a1a1a",
                border: "1px solid #3f6212",
                borderRadius: "8px",
              }}
            />
            <Legend />
          </PieChart>
        </ResponsiveContainer>
        <div className="mt-4 space-y-2">
          <div className="grid grid-cols-2 gap-2 text-sm">
            {chartData.map((item, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <div
                  className="h-3 w-3 rounded-full"
                  style={{ backgroundColor: item.color }}
                />
                <span className="text-muted-foreground truncate">{item.name}</span>
                <span className="ml-auto font-mono">
                  {title === "Token Distribution" ? `${item.value} vnodes` : `${item.value} keys`}
                </span>
              </div>
            ))}
          </div>
          {totalNodes && virtualNodesPerNode && (
            <div className="mt-4 pt-4 border-t text-xs text-muted-foreground">
              <p>Total nodes: {totalNodes}</p>
              <p>Virtual nodes per node: {virtualNodesPerNode}</p>
              <p>Total virtual nodes: {totalNodes * virtualNodesPerNode}</p>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
