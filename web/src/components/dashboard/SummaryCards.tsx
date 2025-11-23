"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Activity, Server, Database } from "lucide-react";
import { motion } from "framer-motion";

interface SummaryCardsProps {
  health: string;
  activeNodes: number;
  totalKeys: number;
}

export function SummaryCards({ health, activeNodes, totalKeys }: SummaryCardsProps) {
  const healthColor =
    health === "healthy"
      ? "text-green-500"
      : health === "degraded"
      ? "text-yellow-500"
      : health === "critical"
      ? "text-red-500"
      : "text-gray-500";

  const healthBg =
    health === "healthy"
      ? "bg-green-500/10"
      : health === "degraded"
      ? "bg-yellow-500/10"
      : health === "critical"
      ? "bg-red-500/10"
      : "bg-gray-500/10";

  return (
    <div className="grid gap-4 md:grid-cols-3">
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, delay: 0 }}
      >
        <Card className="border-lime-500/20 hover:border-lime-500/40 transition-colors">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Cluster Health</CardTitle>
            <Activity className={`h-4 w-4 ${healthColor}`} />
          </CardHeader>
          <CardContent>
            <div className={`text-2xl font-bold ${healthColor} capitalize`}>{health}</div>
            <div className={`mt-2 h-2 rounded-full ${healthBg} overflow-hidden`}>
              <motion.div
                className={`h-full ${healthColor.replace('text-', 'bg-')}`}
                initial={{ width: "0%" }}
                animate={{ width: health === "healthy" ? "100%" : health === "degraded" ? "60%" : "30%" }}
                transition={{ duration: 0.5, delay: 0.2 }}
              />
            </div>
          </CardContent>
        </Card>
      </motion.div>

      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, delay: 0.1 }}
      >
        <Card className="hover:border-primary/40 transition-colors">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Nodes</CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activeNodes}</div>
            <p className="text-xs text-muted-foreground mt-1">
              {activeNodes === 1 ? "node" : "nodes"} online
            </p>
          </CardContent>
        </Card>
      </motion.div>

      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, delay: 0.2 }}
      >
        <Card className="hover:border-primary/40 transition-colors">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Keys</CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalKeys.toLocaleString()}</div>
            <p className="text-xs text-muted-foreground mt-1">
              {totalKeys === 0 ? "No data stored" : "keys stored"}
            </p>
          </CardContent>
        </Card>
      </motion.div>
    </div>
  );
}
