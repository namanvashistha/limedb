"use client";

import { RingState, RingRange } from "@/lib/types";
import { Disc } from "lucide-react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";

// Use explicit hex values — Tailwind JIT purges dynamically constructed class names
const COLORS = [
  "#84cc16", // lime-500
  "#3b82f6", // blue-500
  "#a855f7", // purple-500
  "#06b6d4", // cyan-500
  "#f59e0b", // amber-500
  "#f43f5e", // rose-500
  "#10b981", // emerald-500
  "#ec4899", // pink-500
  "#8b5cf6", // violet-500
  "#14b8a6", // teal-500
  "#f97316", // orange-500
  "#06b6d4", // cyan-600
  "#6366f1", // indigo-500
  "#84cc16", // lime-600
  "#d946ef", // fuchsia-500
  "#0891b2", // cyan-700
  "#4f46e5", // indigo-600
  "#22d3ee", // cyan-400
  "#a21caf", // fuchsia-600
  "#7c3aed", // violet-600
  "#059669", // emerald-600
  "#ea580c", // orange-600
  "#7c2d12", // orange-900
  "#dc2626", // red-600
  "#2563eb", // blue-600
  "#9333ea", // purple-600
  "#0369a1", // sky-700
  "#16a34a", // green-600
  "#9f1239", // rose-900
  "#881391", // purple-900
];

export function KeyspaceMap({ data }: { data: RingState }) {
  const rangesMap = data.ranges || {};
  // Derive a stable sorted node list from the actual ranges keys
  const allNodes = Object.keys(rangesMap).sort();

  if (allNodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-48 text-muted-foreground">
        <Disc className="h-10 w-10 opacity-20 mb-2" />
        <p className="text-sm">No nodes in ring</p>
      </div>
    );
  }

  // 1. Flatten all ranges, injecting the node key (Go API omits it from each range object)
  const allRanges: RingRange[] = [];
  Object.entries(rangesMap).forEach(([nodeUrl, nodeRanges]) => {
    nodeRanges.forEach(r => allRanges.push({ ...r, node: nodeUrl }));
  });
  const sortedRanges = allRanges.sort((a, b) => a.start - b.start);

  // xxhash produces uint64 values — use BigInt to avoid float precision loss.
  // JSON parses large numbers as floats, so we re-parse via string carefully.
  const MAX_RING = 18446744073709551616n; // 2^64 (full ring size)

  const toBigInt = (v: number) => BigInt(Math.round(v));
  const pctOf = (size: number) =>
    (Number((toBigInt(size) * 100000n) / MAX_RING) / 1000).toFixed(2);

  // 2. Stable color index keyed by sorted node URL
  const nodeColorIdx: Record<string, number> = {};
  allNodes.forEach((node, i) => {
    nodeColorIdx[node] = i;
  });

  // Calculate total size for coverage label
  const totalCoveredBig = sortedRanges.reduce((sum, r) => sum + toBigInt(r.size), 0n);
  const totalCoveredPct = (Number((totalCoveredBig * 1000n) / MAX_RING) / 10).toFixed(1);

  return (
    <div className="space-y-6">
      <div>
        <div className="flex items-center justify-between mb-2">
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Keyspace Distribution</p>
          <span className="text-xs text-muted-foreground">{totalCoveredPct}% mapped</span>
        </div>

        {/* ── Continuous Bar ────────────────────────────────────────── */}
        <div className="w-full h-8 rounded-lg overflow-hidden flex border bg-muted/40 divide-x divide-background/10">
          <TooltipProvider delayDuration={100}>
            {sortedRanges.map((range, i) => {
              const colorIdx = nodeColorIdx[range.node] ?? 0;
              const color = COLORS[colorIdx % COLORS.length];
              const pct = parseFloat(pctOf(range.size));

              // Ensure minimal width so it's clickable/hoverable even if <0.5%
              const displayPct = Math.max(pct, 0.5);

              return (
                <Tooltip key={`${range.node}-${range.start}-${i}`}>
                  <TooltipTrigger asChild>
                    <div
                      className="h-full transition-opacity hover:opacity-80 cursor-pointer"
                      style={{ width: `${displayPct}%`, backgroundColor: color }}
                    />
                  </TooltipTrigger>
                  <TooltipContent side="top" className="text-xs p-2">
                    <p className="font-mono font-bold">{range.node}</p>
                    <p className="text-muted-foreground">Size: {pctOf(range.size)}%</p>
                  </TooltipContent>
                </Tooltip>
              );
            })}
          </TooltipProvider>
        </div>
      </div>

      {/* ── Legend & Ownership List ────────────────────────────────── */}
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        {allNodes.map((node, i) => {
          const color = COLORS[i % COLORS.length];
          const nodeRanges = rangesMap[node] || [];
          const nodeSize = nodeRanges.reduce((sum, r) => sum + r.size, 0);
          const pct = pctOf(nodeSize);

          return (
            <div key={node} className="flex items-center gap-2 p-2 rounded-md bg-muted/20 border border-border/50">
              <div className="w-3 h-3 rounded-full flex-shrink-0" style={{ backgroundColor: color }} />
              <div className="min-w-0 flex-1">
                <p className="text-xs font-mono truncate">{node.replace(/^https?:\/\//, "")}</p>
                <p className="text-xs font-bold" style={{ color }}>{pct}% <span className="text-[10px] text-muted-foreground font-normal">({nodeRanges.length} ranges)</span></p>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
