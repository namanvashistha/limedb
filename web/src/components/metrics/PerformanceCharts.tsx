"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from "recharts";
import { useEffect, useState } from "react";
import { Activity, Cpu, HardDrive, Timer } from "lucide-react";

interface MetricData {
  timestamp: string;
  cpu: number;
  memory: number;
  latency: number;
}

export function PerformanceCharts() {
  const [data, setData] = useState<MetricData[]>([]);
  const [currentMetrics, setCurrentMetrics] = useState({
    cpu: 0,
    memory: 0,
    latency: 0,
  });

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        // Simulate metrics - in real app, fetch from /api/proxy/health or metrics endpoint
        const newPoint: MetricData = {
          timestamp: new Date().toLocaleTimeString(),
          cpu: Math.random() * 100,
          memory: Math.random() * 100,
          latency: Math.random() * 50,
        };

        setCurrentMetrics({
          cpu: newPoint.cpu,
          memory: newPoint.memory,
          latency: newPoint.latency,
        });

        setData((prev) => {
          const updated = [...prev, newPoint];
          // Keep only last 20 data points
          return updated.slice(-20);
        });
      } catch (error) {
        console.error("Failed to fetch metrics:", error);
      }
    };

    fetchMetrics();
    const interval = setInterval(fetchMetrics, 2000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="grid gap-4 md:grid-cols-3 mb-4">
      {/* Current Metrics Cards */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">CPU Usage</CardTitle>
          <Cpu className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{currentMetrics.cpu.toFixed(1)}%</div>
          <p className="text-xs text-muted-foreground">Real-time CPU utilization</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Memory</CardTitle>
          <HardDrive className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{currentMetrics.memory.toFixed(1)}%</div>
          <p className="text-xs text-muted-foreground">Memory consumption</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Latency</CardTitle>
          <Timer className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{currentMetrics.latency.toFixed(1)} ms</div>
          <p className="text-xs text-muted-foreground">Request latency</p>
        </CardContent>
      </Card>

      {/* Performance Chart */}
      <Card className="col-span-3">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Live Performance Metrics
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={data}>
              <CartesianGrid strokeDasharray="3 3" stroke="#333" />
              <XAxis dataKey="timestamp" stroke="#888" fontSize={12} />
              <YAxis stroke="#888" fontSize={12} />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1a1a1a",
                  border: "1px solid #3f6212",
                  borderRadius: "8px",
                }}
              />
              <Legend />
              <Line
                type="monotone"
                dataKey="cpu"
                stroke="#84cc16"
                strokeWidth={2}
                dot={false}
                name="CPU %"
              />
              <Line
                type="monotone"
                dataKey="memory"
                stroke="#3b82f6"
                strokeWidth={2}
                dot={false}
                name="Memory %"
              />
              <Line
                type="monotone"
                dataKey="latency"
                stroke="#a855f7"
                strokeWidth={2}
                dot={false}
                name="Latency (ms)"
              />
            </LineChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>
    </div>
  );
}
