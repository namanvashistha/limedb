"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Wifi, WifiOff } from "lucide-react";
import { motion } from "framer-motion";

interface DiscoveredNodesProps {
  nodes: string[];
  currentNode: string | null;
}

export function DiscoveredNodes({ nodes, currentNode }: DiscoveredNodesProps) {
  if (nodes.length === 0) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span className="flex items-center gap-2">
            <Wifi className="h-5 w-5" />
            Discovered Nodes
          </span>
          <Badge variant="outline">{nodes.length} nodes</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ScrollArea className="h-[200px]">
          <div className="space-y-2">
            {nodes.map((nodeUrl, idx) => {
              const isCurrent = nodeUrl === currentNode;
              return (
                <motion.div
                  key={nodeUrl}
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: idx * 0.05 }}
                  className={`flex items-center justify-between p-3 rounded-lg border ${
                    isCurrent ? "bg-lime-500/10 border-lime-500" : "bg-muted/50"
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`h-2 w-2 rounded-full ${isCurrent ? "bg-lime-500 animate-pulse" : "bg-blue-500"}`} />
                    <span className="font-mono text-sm">{nodeUrl}</span>
                  </div>
                  {isCurrent && (
                    <Badge variant="default" className="bg-lime-500">
                      Current
                    </Badge>
                  )}
                </motion.div>
              );
            })}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  );
}
