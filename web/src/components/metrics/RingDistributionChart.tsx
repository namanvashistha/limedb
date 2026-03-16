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

export function RingDistributionChart({ data }: { data: RingState }) {
  const allNodes = data.allNodes || [];
  
  if (allNodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Disc className="h-12 w-12 opacity-20 mb-2" />
        <p>No nodes in ring</p>
      </div>
    );
  }

  // Calculate actual ring ownership
  const ownershipMap: Record<string, number> = {};
  
  if (data.ranges) {
    Object.values(data.ranges).forEach(ranges => {
      ranges.forEach(range => {
        if (!ownershipMap[range.node]) {
          ownershipMap[range.node] = 0;
        }
        ownershipMap[range.node] += range.size;
      });
    });
  }

  // Fallback if ranges aren't populated yet
  const chartData = allNodes.map((node: string, index: number) => {
    const rawValue = ownershipMap[node] || 0;
    // The hash ring is 2^32
    const MAX_RING = 4294967295;
    const percentage = rawValue > 0 ? (rawValue / MAX_RING) * 100 : (100 / allNodes.length);
    
    return {
      name: node,
      value: percentage, // Recharts Pie handles percentages relative to sum naturally
      tooltipValue: `${percentage.toFixed(2)}%`,
      color: COLORS[index % COLORS.length],
    };
  });

  return (
    <div className="h-full flex flex-col">
      <div className="flex-1 min-h-0">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              innerRadius={60}
              outerRadius={80}
              paddingAngle={2}
              dataKey="value"
            >
              {chartData.map((entry: { color: string }, index: number) => (
                <Cell key={`cell-${index}`} fill={entry.color} strokeWidth={0} />
              ))}
            </Pie>
            <Tooltip 
              contentStyle={{ backgroundColor: "#1f2937", border: "none", borderRadius: "8px", color: "#f3f4f6" }}
              itemStyle={{ color: "#f3f4f6" }}
              formatter={(value: any, name: any, props: any) => [props.payload.tooltipValue, "Ownership"]}
            />
            <Legend verticalAlign="bottom" height={36}/>
          </PieChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}


