"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { GossipMetrics } from "@/lib/types";

export function ClusterStatusBar() {
  const [gossip, setGossip] = useState<GossipMetrics | null>(null);
  const [rps, setRps] = useState(0);
  const [connected, setConnected] = useState(true);

  useEffect(() => {
    let mounted = true;
    const poll = async () => {
      try {
        const g = await api.getGossipMetrics();
        if (!mounted) return;
        setGossip(g);
        setConnected(true);

        // Also pull health from seed for live RPS
        const h = await api.getHealth(g.node_url).catch(() => null);
        if (mounted && h) setRps(h.requests_per_second || 0);
      } catch {
        if (mounted) setConnected(false);
      }
    };
    poll();
    const iv = setInterval(poll, 3000);
    return () => { mounted = false; clearInterval(iv); };
  }, []);

  const isHealthy = connected && gossip?.cluster_health === "healthy";
  const totalNodes = gossip ? gossip.active_peers + 1 : 0;

  return (
    <div className="px-4 py-3 border-t flex flex-col gap-1">
      {/* Status dot + label */}
      <div className="flex items-center gap-2">
        <span className={`w-2 h-2 rounded-full flex-shrink-0 ${
          !connected ? "bg-muted-foreground/40" :
          isHealthy   ? "bg-green-500 shadow-[0_0_6px_#22c55e]" :
                        "bg-red-500 shadow-[0_0_6px_#ef4444]"
        }`} />
        <span className={`text-xs font-medium ${
          !connected ? "text-muted-foreground" :
          isHealthy   ? "text-green-600 dark:text-green-400" :
                        "text-red-500"
        }`}>
          {!connected ? "Disconnected" : isHealthy ? "Cluster healthy" : "Degraded"}
        </span>
      </div>

      {/* Mini stats row */}
      {connected && gossip && (
        <div className="flex items-center gap-3 text-[10px] text-muted-foreground tabular-nums">
          <span>{totalNodes} node{totalNodes !== 1 ? "s" : ""}</span>
          <span className="opacity-30">·</span>
          <span>{rps.toFixed(1)} req/s</span>
          {gossip.dead_peers > 0 && (
            <>
              <span className="opacity-30">·</span>
              <span className="text-red-500">{gossip.dead_peers} dead</span>
            </>
          )}
        </div>
      )}
    </div>
  );
}
