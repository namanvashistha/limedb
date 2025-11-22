import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { NodeStatus } from "@/lib/types";

interface ClusterStatusTableProps {
  node: NodeStatus | null;
}

export function ClusterStatusTable({ node }: ClusterStatusTableProps) {
  if (!node) {
    return (
      <div className="rounded-md border p-8 text-center text-muted-foreground">
        No cluster data available
      </div>
    );
  }

  const isActive = node.status === "active";

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Status</TableHead>
            <TableHead>Node URL</TableHead>
            <TableHead className="text-right">Peers</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>
              {isActive ? (
                <Badge variant="default" className="bg-green-500 hover:bg-green-600">
                  Active
                </Badge>
              ) : (
                <Badge variant="destructive">Down</Badge>
              )}
            </TableCell>
            <TableCell className="font-medium">{node.nodeUrl}</TableCell>
            <TableCell className="text-right">{node.peers?.length || 0}</TableCell>
          </TableRow>
          {node.peers && node.peers.length > 0 && (
            <>
              <TableRow>
                <TableCell colSpan={3} className="bg-muted/50 font-semibold">
                  Peer Nodes
                </TableCell>
              </TableRow>
              {node.peers.map((peer, idx) => (
                <TableRow key={idx}>
                  <TableCell>
                    <Badge variant="outline">Peer</Badge>
                  </TableCell>
                  <TableCell className="font-medium">{peer}</TableCell>
                  <TableCell className="text-right">-</TableCell>
                </TableRow>
              ))}
            </>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
