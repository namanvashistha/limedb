import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { NodeStatus } from "@/lib/types";
import { Server, Star } from "lucide-react";

interface ClusterStatusTableProps {
  nodes: Record<string, NodeStatus>;
}

export function ClusterStatusTable({ nodes }: ClusterStatusTableProps) {
  const nodesList = Object.values(nodes);
  
  // Sort so self node appears first
  const sortedNodes = nodesList.sort((a, b) => {
    if (a.isSelf) return -1;
    if (b.isSelf) return 1;
    return 0;
  });
  
  if (sortedNodes.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Server className="h-5 w-5" />
            Cluster Nodes
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col items-center justify-center h-64 text-muted-foreground space-y-2">
            <Server className="h-12 w-12 opacity-20" />
            <p>No cluster data available</p>
            <p className="text-xs">Waiting for gossip discovery...</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Server className="h-5 w-5" />
          Cluster Nodes
          <Badge variant="outline" className="ml-auto">{sortedNodes.length}</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Type</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Node URL</TableHead>
                <TableHead className="text-right">Peers</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedNodes.map((node) => {
                const isActive = node.status === "active";
                const isSelf = node.isSelf;
                return (
                  <TableRow 
                    key={node.nodeUrl} 
                    className={`hover:bg-muted/50 transition-colors ${isSelf ? 'bg-lime-500/5' : ''}`}
                  >
                    <TableCell>
                      {isSelf ? (
                        <Badge variant="outline" className="border-lime-500 text-lime-500">
                          <Star className="h-3 w-3 mr-1 fill-lime-500" />
                          Self
                        </Badge>
                      ) : (
                        <Badge variant="secondary">
                          Peer
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      {isActive ? (
                        <Badge variant="default" className="bg-green-500 hover:bg-green-600">
                          <div className="flex items-center gap-1">
                            <div className="h-2 w-2 rounded-full bg-white animate-pulse" />
                            Active
                          </div>
                        </Badge>
                      ) : node.status === "stale" ? (
                        <Badge variant="secondary" className="bg-yellow-500">
                          Stale
                        </Badge>
                      ) : (
                        <Badge variant="destructive">
                          {node.status || "Down"}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-sm">{node.nodeUrl}</TableCell>
                    <TableCell className="text-right font-mono">{node.peers?.length || 0}</TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}
