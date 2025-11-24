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
import { Server } from "lucide-react";

interface ClusterStatusTableProps {
  nodes: Record<string, NodeStatus>;
}

export function ClusterStatusTable({ nodes }: ClusterStatusTableProps) {
  const nodesList = Object.values(nodes);
  
  // Sort alphabetically by URL - ALL NODES EQUAL, no self-node priority
  const sortedNodes = nodesList.sort((a, b) => {
    return a.nodeUrl.localeCompare(b.nodeUrl);
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
                // NO SPECIAL TREATMENT - all nodes are peers
                return (
                  <TableRow 
                    key={node.nodeUrl} 
                    className="hover:bg-muted/50 transition-colors"
                  >
                    <TableCell>
                      <Badge variant="secondary">
                        Peer
                      </Badge>
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
