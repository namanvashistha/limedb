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
  nodes: Record<string, NodeStatus>;
}

export function ClusterStatusTable({ nodes }: ClusterStatusTableProps) {
  const sortedUrls = Object.keys(nodes).sort();

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Status</TableHead>
            <TableHead>Node URL</TableHead>
            <TableHead>Peers</TableHead>
            <TableHead className="text-right">Last Updated</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sortedUrls.map((url) => {
            const node = nodes[url];
            const isDown = !node || !node.status || node.status === "dead"; // Simple check, adjust based on actual data structure
            // Actually, if node is missing from map it might be down, or if status is 'dead'
            // The API returns { "url": { ... } } or { "url": { "error": ... } } in TUI logic.
            // Our types say NodeStatus has status field.
            
            // Let's handle the case where node might be an error object if we didn't type it strictly enough
            // But assuming valid NodeStatus for now.
            
            return (
              <TableRow key={url}>
                <TableCell>
                  {node.status === "active" ? (
                    <Badge variant="default" className="bg-green-500 hover:bg-green-600">Active</Badge>
                  ) : (
                    <Badge variant="destructive">Down</Badge>
                  )}
                </TableCell>
                <TableCell className="font-medium">{url}</TableCell>
                <TableCell>{node.peers?.length || 0}</TableCell>
                <TableCell className="text-right">
                  {new Date(node.timestamp).toLocaleTimeString()}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
