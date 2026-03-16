"use client";

import { Handle, Position } from "@xyflow/react";
import type { NodeProps } from "@xyflow/react";
import { Badge } from "@/components/ui/badge";
import { Server, Activity, Clock, Users } from "lucide-react";

export interface ClusterNodeData extends Record<string, unknown> {
  url: string;
  status: "active" | "stale" | "dead";
  peers: number;
  heartbeat: number;
  generation: number;
  lag: number;
  isSeed?: boolean;
}

export function ClusterNode(props: any) {
  const data = props.data as ClusterNodeData;
  
  const isActive = data.status === "active";
  const isStale = data.status === "stale";
  const isDead = data.status === "dead";

  const statusColor = isActive ? "bg-green-500" : isStale ? "bg-yellow-500" : "bg-red-500";
  const borderColor = isActive ? "border-green-500/50" : isStale ? "border-yellow-500/50" : "border-red-500/50";
  const bgGradient = isActive 
    ? "from-green-500/10 to-green-500/5" 
    : isStale 
    ? "from-yellow-500/10 to-yellow-500/5" 
    : "from-red-500/10 to-red-500/5";

  const hostname = data.url.split("//")[1] || data.url;

  return (
    <>
      {/* Connection handles on all 4 sides - invisible */}
      <Handle type="source" position={Position.Top} id="top" className="!bg-transparent !w-1 !h-1 !border-0" />
      <Handle type="source" position={Position.Right} id="right" className="!bg-transparent !w-1 !h-1 !border-0" />
      <Handle type="source" position={Position.Bottom} id="bottom" className="!bg-transparent !w-1 !h-1 !border-0" />
      <Handle type="source" position={Position.Left} id="left" className="!bg-transparent !w-1 !h-1 !border-0" />
      
      <Handle type="target" position={Position.Top} id="top" className="!bg-transparent !w-1 !h-1 !border-0" />
      <Handle type="target" position={Position.Right} id="right" className="!bg-transparent !w-1 !h-1 !border-0" />
      <Handle type="target" position={Position.Bottom} id="bottom" className="!bg-transparent !w-1 !h-1 !border-0" />
      <Handle type="target" position={Position.Left} id="left" className="!bg-transparent !w-1 !h-1 !border-0" />
      
      <div className={`
        group relative px-4 py-3 rounded-xl border-2 ${borderColor}
        bg-gradient-to-br ${bgGradient} backdrop-blur-sm
        shadow-lg hover:shadow-xl transition-all duration-300
        min-w-[240px]
        hover:scale-105
      `}>
        {/* Status pulse indicator */}
        <div className="absolute -top-1 -right-1">
          <div className={`relative w-3 h-3 ${statusColor} rounded-full`}>
            {isActive && (
              <div className={`absolute inset-0 ${statusColor} rounded-full animate-ping opacity-75`} />
            )}
          </div>
        </div>

        {/* Header: URL and Badge */}
        <div className="flex items-center gap-2 mb-3">
          <div className={`p-1.5 rounded-md ${isActive ? "bg-green-500/20" : "bg-muted"}`}>
            <Server className={`h-4 w-4 ${isActive ? "text-green-600" : "text-muted-foreground"}`} />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-semibold text-sm truncate">{hostname}</span>
              {data.isSeed && (
                <Badge variant="secondary" className="text-xs px-1.5 py-0">
                  seed
                </Badge>
              )}
            </div>
          </div>
          <Badge 
            variant={isActive ? "outline" : "destructive"}
            className={`text-xs ${isActive ? "text-green-600 border-green-600" : ""}`}
          >
            {data.status}
          </Badge>
        </div>

        {/* Metrics */}
        <div className="flex items-center gap-3 text-xs text-muted-foreground border-t pt-2">
          <div className="flex items-center gap-1" title="Peer count">
            <Users className="h-3 w-3" />
            <span>{data.peers}</span>
          </div>
          <div className="flex items-center gap-1" title="Gossip round (version)">
            <Activity className="h-3 w-3" />
            <span>{data.heartbeat}</span>
          </div>
          <div className="flex items-center gap-1" title="Consensus lag (rounds behind)">
            <Clock className="h-3 w-3" />
            <span>{data.lag > 0 ? `Δ${data.lag}` : "✔"}</span>
          </div>
        </div>
        {data.generation > 0 && (
          <div className="text-[10px] text-muted-foreground/60 mt-1 font-mono">
            gen {data.generation}
          </div>
        )}
      </div>
    </>
  );
}
