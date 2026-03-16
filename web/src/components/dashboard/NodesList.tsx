import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { api } from "@/lib/api";
import { GossipMetrics } from "@/lib/types";
import { AlertCircle, ArrowRight, Server, Activity, Clock, Search } from "lucide-react";
import { Input } from "@/components/ui/input";

interface NodesListProps {
  onSelectNode: (nodeUrl: string) => void;
  selectedNodeUrl?: string | null;
  compact?: boolean;
}

export function NodesList({ onSelectNode, selectedNodeUrl, compact = false }: NodesListProps) {
  const [metrics, setMetrics] = useState<GossipMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const data = await api.getGossipMetrics();
        setMetrics(data);
        setError(null);
      } catch (err) {
        // console.warn("Failed to fetch nodes", err);
        setError("Failed to fetch node list");
      } finally {
        setLoading(false);
      }
    };

    fetchMetrics();
    const interval = setInterval(fetchMetrics, 2000);
    return () => clearInterval(interval);
  }, []);

  if (loading && !metrics) {
    return <div className="flex items-center justify-center h-full">Loading nodes...</div>;
  }

  if (error && !metrics) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-destructive gap-4">
        <AlertCircle className="h-12 w-12" />
        <div className="text-lg font-medium">{error}</div>
      </div>
    );
  }

  // Combine seed and peers, sort alphabetically by URL
  const seedNode = metrics ? {
    url: metrics.node_url,
    heartbeat: metrics.node_heartbeat,
    lag: 0, // Seed node has no lag to itself
    status: "active" as const,
    isSeed: true
  } : null;

  const peerNodes = (metrics?.peer_details || []).map(peer => ({
    ...peer,
    isSeed: false
  }));

  // Always sort: combine all nodes and sort alphabetically
  const allNodes = (seedNode ? [seedNode, ...peerNodes] : peerNodes)
    .slice() // Create shallow copy to avoid mutation
    .sort((a, b) => a.url.localeCompare(b.url));
  
  const activeCount = allNodes.filter(n => n.status === "active").length;
  const filteredNodes = allNodes.filter(node => node.url.toLowerCase().includes(searchQuery.toLowerCase()));

  if (compact) {
    return (
      <div className="h-full flex flex-col">
        <div className="pb-3 border-b mb-1">
          <div className="flex items-center justify-between mb-3">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Nodes</p>
            <span className="text-xs text-muted-foreground tabular-nums">{activeCount}/{allNodes.length} up</span>
          </div>
          <div className="relative">
            <Search className="absolute left-2 top-2.5 h-3 w-3 text-muted-foreground" />
            <Input
              placeholder="Search…"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-7 h-7 text-xs border-0 bg-muted/50 focus-visible:ring-0"
            />
          </div>
        </div>
        <ScrollArea className="flex-1">
          <div className="p-2">
            {filteredNodes.map((node) => {
              const pos = node.url.lastIndexOf(":");
              const displayName = pos !== -1 ? node.url.substring(pos) : node.url;
              const isSelected = selectedNodeUrl === node.url;
              const isActive = node.status === "active";
              
              return (
                <div
                  key={node.url}
                  onClick={() => onSelectNode(node.url)}
                  className={`
                    group flex items-center justify-between p-2 mb-1 rounded-md cursor-pointer transition-colors text-sm
                    ${isSelected 
                      ? "bg-primary text-primary-foreground font-medium" 
                      : "hover:bg-muted font-normal text-muted-foreground"
                    }
                  `}
                >
                  <div className="flex items-center gap-2 overflow-hidden">
                    <div className={`w-2 h-2 rounded-full flex-shrink-0 ${isActive ? (isSelected ? "bg-primary-foreground" : "bg-green-500") : "bg-destructive"}`} />
                    <span className="truncate">{displayName}</span>
                    {node.isSeed && (
                      <span className={`text-[10px] px-1 rounded-sm border ${isSelected ? "border-primary-foreground/30 text-primary-foreground" : "border-border text-muted-foreground"}`}>seed</span>
                    )}
                  </div>
                  
                  <div className={`text-xs ml-2 flex-shrink-0 opacity-80 flex items-center gap-1`}>
                    {!isSelected && <Clock className="h-3 w-3" />}
                    {node.lag}ms
                  </div>
                </div>
              );
            })}
            {filteredNodes.length === 0 && (
              <div className="flex flex-col items-center justify-center h-32 text-muted-foreground">
                <Server className="h-8 w-8 mb-2 opacity-20" />
                <p className="text-sm">No nodes found</p>
                {searchQuery && <p className="text-xs mt-1">Try clearing your search</p>}
              </div>
            )}
          </div>
        </ScrollArea>
      </div>
    );
  }

  return (
    <Card className="h-full flex flex-col">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Cluster Nodes ({allNodes.length})</CardTitle>
          <div className="relative w-64">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Filter nodes..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 h-9"
            />
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex-1 overflow-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Node URL</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Heartbeat</TableHead>
              <TableHead className="text-right">Lag (ms)</TableHead>
              <TableHead className="text-right">Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredNodes.map((node) => (
              <TableRow 
                key={node.url} 
                className={`cursor-pointer hover:bg-muted/50 ${selectedNodeUrl === node.url ? "bg-muted border-l-4 border-l-primary" : ""}`}
                onClick={() => onSelectNode(node.url)}
              >
                <TableCell className="font-mono font-medium py-3">
                  <div className="flex items-center gap-2">
                    {node.url}
                    {node.isSeed && (
                      <Badge variant="secondary" className="text-xs">
                        seed
                      </Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell className="py-3">
                  <Badge 
                    variant={node.status === "active" ? "outline" : "destructive"}
                    className={node.status === "active" ? "text-green-500 border-green-500" : ""}
                  >
                    {node.status}
                  </Badge>
                </TableCell>
                <TableCell className="text-right font-mono">{node.heartbeat}</TableCell>
                <TableCell className="text-right font-mono">{node.lag}</TableCell>
                <TableCell className="text-right">
                  <Button variant="ghost" size="sm" onClick={(e) => {
                    e.stopPropagation();
                    onSelectNode(node.url);
                  }}>
                    <ArrowRight className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {filteredNodes.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center h-24 text-muted-foreground">
                  No nodes found via gossip.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
