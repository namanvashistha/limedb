"use client";

import { useState } from "react";
import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from "recharts";
import { RingState } from "@/lib/types";
import { GossipMetrics } from "@/lib/types";
import { Disc, ChevronDown, ChevronUp } from "lucide-react";

interface RingDistributionChartProps {
  ring: RingState;
  gossip: GossipMetrics | null;
}

const COLORS = ["#84cc16", "#3b82f6", "#a855f7", "#06b6d4", "#eab308", "#ef4444"];

export function RingDistributionChart({ data }: { data: RingState }) {
  const [legendOpen, setLegendOpen] = useState(true);
  const allNodes = (data.allNodes || []).sort();
  
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
  const totalRawSize = Object.values(ownershipMap).reduce((a, b) => a + b, 0);

  const chartData = allNodes.map((node: string, index: number) => {
    const rawValue = ownershipMap[node] || 0;
    // Use relative proportions (sum of all sizes = full ring, regardless of ring width)
    const percentage = totalRawSize > 0
      ? (rawValue / totalRawSize) * 100
      : (100 / allNodes.length);
    
    return {
      name: node,
      value: percentage,
      tooltipValue: `${percentage.toFixed(1)}%`,
      color: COLORS[index % COLORS.length],
    };
  });

  return (
    <div className="h-full flex flex-col">
      <div className="flex-1 min-h-0 relative">
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
          </PieChart>
        </ResponsiveContainer>
      </div>

      {/* Collapsible legend */}
      <div className="border-t relative z-10 bg-background">
        <button
          onClick={() => setLegendOpen(o => !o)}
          className="w-full flex items-center justify-between px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          <span>Legend</span>
          {legendOpen ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />}
        </button>
        {legendOpen && (
          <div className="flex flex-wrap gap-x-3 gap-y-1 px-3 pb-2">
            {chartData.map((entry: { name: string; color: string; tooltipValue: string }) => (
              <div key={entry.name} className="flex items-center gap-1 text-xs text-muted-foreground">
                <div className="w-2 h-2 rounded-sm flex-shrink-0" style={{ backgroundColor: entry.color }} />
                <span className="truncate max-w-[120px]" title={entry.name}>{entry.name}</span>
                <span className="text-muted-foreground/60">{entry.tooltipValue}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}


