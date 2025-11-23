"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PieChart, Pie, Cell, ResponsiveContainer, Legend, Tooltip } from "recharts";
import { RingState } from "@/lib/types";
import { Disc } from "lucide-react";

interface RingDistributionChartProps {
  ring: RingState;
}

const COLORS = ["#84cc16", "#3b82f6", "#a855f7", "#06b6d4", "#eab308", "#ef4444"];

export function RingDistributionChart({ ring }: RingDistributionChartProps) {
  console.log("Ring data received:", ring);
  
  const ranges = ring.ranges || {};
  console.log("Ranges:", ranges);
  
  const nodes = Object.keys(ranges);
  console.log("Nodes in ranges:", nodes);
  
  const chartData = nodes.map((node, index) => {
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
  
  console.log("Chart data:", chartData);

  if (chartData.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Disc className="h-5 w-5" />
            Ring Distribution
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center justify-center h-64 text-muted-foreground">
          <p>No ring data available</p>
          <p className="text-xs mt-2">Ring has {nodes.length} nodes but no key distribution</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Disc className="h-5 w-5" />
          Token Ring Distribution
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
              label={({ name, percent }) => `${name}: ${((percent ?? 0) * 100).toFixed(0)}%`}
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
        <div className="mt-4 grid grid-cols-2 gap-2 text-sm">
          {chartData.map((item, idx) => (
            <div key={idx} className="flex items-center gap-2">
              <div
                className="h-3 w-3 rounded-full"
                style={{ backgroundColor: item.color }}
              />
              <span className="text-muted-foreground truncate">{item.name}</span>
              <span className="ml-auto font-mono">{item.value.toLocaleString()}</span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
