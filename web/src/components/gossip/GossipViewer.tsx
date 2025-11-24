"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { GossipMetrics } from "@/lib/types";
import { Network, Heart, AlertTriangle } from "lucide-react";

interface GossipViewerProps {
  gossip: GossipMetrics | null;
}

export function GossipViewer({ gossip }: GossipViewerProps) {
  if (!gossip) {
    return (
      <div className="grid gap-4">
        <Card>
          <CardContent className="flex items-center justify-center h-64 text-muted-foreground">
            No gossip data available
          </CardContent>
        </Card>
      </div>
    );
  }

  const healthColor =
    gossip.cluster_health === "healthy"
      ? "text-green-500"
      : gossip.cluster_health === "degraded"
      ? "text-yellow-500"
      : "text-red-500";

  return (
    <div className="grid gap-4">
      {/* Summary Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Cluster Health</CardTitle>
            <Heart className={`h-4 w-4 ${healthColor}`} />
          </CardHeader>
          <CardContent>
            <div className={`text-2xl font-bold ${healthColor}`}>
              {gossip.cluster_health.toUpperCase()}
            </div>
            <p className="text-xs text-muted-foreground">
              Heartbeat: {gossip.node_heartbeat}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Peers</CardTitle>
            <Network className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {gossip.active_peers}/{gossip.total_peers}
            </div>
            <p className="text-xs text-muted-foreground">
              Dead: {gossip.dead_peers} | Stale: {gossip.stale_peers}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Convergence</CardTitle>
            <AlertTriangle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{gossip.convergence_rate.toFixed(1)}%</div>
            <p className="text-xs text-muted-foreground">
              Avg Lag: {gossip.average_lag.toFixed(1)} | Max: {gossip.max_lag}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Peer Details Table */}
      <Card>
        <CardHeader>
          <CardTitle>Peer Details</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Peer URL</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Heartbeat</TableHead>
                <TableHead className="text-right">Lag</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {gossip.peer_details && gossip.peer_details.length > 0 ? (
                [...gossip.peer_details]
                  .sort((a, b) => a.url.localeCompare(b.url))
                  .map((peer, idx) => (
                  <TableRow key={idx}>
                    <TableCell className="font-medium">{peer.url}</TableCell>
                    <TableCell>
                      {peer.status === "active" ? (
                        <Badge className="bg-green-500">Active</Badge>
                      ) : peer.status === "stale" ? (
                        <Badge className="bg-yellow-500">Stale</Badge>
                      ) : (
                        <Badge variant="destructive">Dead</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">{peer.heartbeat}</TableCell>
                    <TableCell className="text-right">
                      <span
                        className={
                          peer.lag === 0
                            ? "text-green-500"
                            : peer.lag <= 5
                            ? "text-yellow-500"
                            : "text-red-500"
                        }
                      >
                        {peer.lag}
                      </span>
                    </TableCell>
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">
                    No peers available
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
