"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { GossipMetrics, HealthResponse } from "@/lib/types";
import { NodeDetailView } from "@/components/dashboard/NodeDetailView";
import { AlertCircle, Search, Server } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ScrollArea } from "@/components/ui/scroll-area";

interface NodeEntry {
  url: string;
  status: string;
  isSeed: boolean;
  lag: number;
  rps: number;
  latency_ms: number;
}

export function NodesFleetView() {
  const [nodes, setNodes] = useState<NodeEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [selectedUrl, setSelectedUrl] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    const poll = async () => {
      try {
        const gossip: GossipMetrics = await api.getGossipMetrics();
        if (!mounted) return;

        const all = [
          { url: gossip.node_url, isSeed: true, lag: 0, status: "active" },
          ...(gossip.peer_details || []).map(p => ({ url: p.url, isSeed: false, lag: p.lag, status: p.status })),
        ];

        const healths = await Promise.all(all.map(n => api.getHealth(n.url).catch(() => null)));
        if (!mounted) return;

        const merged: NodeEntry[] = all.map((n, i) => {
          const h: HealthResponse | null = healths[i];
          return {
            url: n.url,
            status: n.status,
            isSeed: n.isSeed,
            lag: n.lag,
            rps: h?.requests_per_second || 0,
            latency_ms: h?.average_latency_ms || 0,
          };
        });

        setNodes(prev => {
          // Auto-select first node on initial load only
          if (prev.length === 0 && merged.length > 0) {
            setSelectedUrl(merged[0].url);
          }
          return merged;
        });
        setError(null);
      } catch {
        if (mounted) setError("Could not reach cluster");
      } finally {
        if (mounted) setLoading(false);
      }
    };

    poll();
    const iv = setInterval(poll, 3000);
    return () => { mounted = false; clearInterval(iv); };
  }, []);

  if (loading && nodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
        <Server className="h-10 w-10 opacity-20 animate-pulse" />
        <p className="text-sm">Discovering nodes…</p>
      </div>
    );
  }

  if (error && nodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-4">
        <AlertCircle className="h-10 w-10 text-destructive/40" />
        <div className="text-center">
          <p className="font-semibold">Cannot reach cluster</p>
          <p className="text-xs text-muted-foreground mt-1">{error}</p>
          <Button variant="outline" size="sm" className="mt-3" onClick={() => window.location.reload()}>Retry</Button>
        </div>
      </div>
    );
  }

  const filtered = nodes.filter(n => n.url.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="flex gap-0 h-[calc(100vh-8rem)] -mx-6 border-t">
      {/* ── Left Pane: Node List ─────────────────────────────────────── */}
      <div className="w-64 flex-shrink-0 border-r flex flex-col">
        {/* Header */}
        <div className="px-4 py-3 border-b">
          <div className="flex items-center justify-between mb-2">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Nodes</p>
            <span className="text-xs text-muted-foreground tabular-nums">
              {nodes.filter(n => n.status === "active").length}/{nodes.length} up
            </span>
          </div>
          <div className="relative">
            <Search className="absolute left-2 top-2 h-3 w-3 text-muted-foreground" />
            <Input
              placeholder="Filter…"
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="pl-6 h-7 text-xs border-0 bg-muted/50 focus-visible:ring-0 rounded-md"
            />
          </div>
        </div>

        {/* Node rows */}
        <ScrollArea className="flex-1">
          <div className="py-1">
            {filtered.map(node => {
              const isActive = node.status === "active";
              const isSelected = selectedUrl === node.url;
              return (
                <button
                  key={node.url}
                  onClick={() => setSelectedUrl(node.url)}
                  className={cn(
                    "w-full text-left px-4 py-3 flex flex-col gap-0.5 border-b last:border-0 transition-colors",
                    isSelected
                      ? "bg-primary/8 border-l-2 border-l-primary"
                      : "hover:bg-muted/40 border-l-2 border-l-transparent"
                  )}
                >
                  <div className="flex items-center gap-2">
                    <div className={cn(
                      "w-1.5 h-1.5 rounded-full flex-shrink-0",
                      isActive ? "bg-green-500" : "bg-destructive/50"
                    )} />
                    <span className="font-mono text-xs truncate flex-1">
                      {node.url.replace(/^https?:\/\//, "")}
                    </span>
                    {node.isSeed && (
                      <span className="text-[9px] px-1 py-0 bg-muted text-muted-foreground rounded font-medium">SEED</span>
                    )}
                  </div>
                  <div className="flex items-center gap-3 pl-3.5 mt-0.5">
                    <span className="text-[10px] text-muted-foreground tabular-nums">{node.rps.toFixed(1)} req/s</span>
                    <span className="text-[10px] text-muted-foreground tabular-nums">{node.latency_ms.toFixed(1)} ms</span>
                  </div>
                </button>
              );
            })}
            {filtered.length === 0 && (
              <p className="text-xs text-muted-foreground text-center py-8">No nodes found</p>
            )}
          </div>
        </ScrollArea>
      </div>

      {/* ── Right Pane: Node Detail ──────────────────────────────────── */}
      <div className="flex-1 overflow-y-auto px-8 py-6">
        {selectedUrl ? (
          <NodeDetailView nodeUrl={selectedUrl} showBackButton={false} />
        ) : (
          <div className="flex flex-col items-center justify-center h-full text-muted-foreground gap-3">
            <Server className="h-12 w-12 opacity-10" />
            <p className="text-sm">Select a node</p>
          </div>
        )}
      </div>
    </div>
  );
}
