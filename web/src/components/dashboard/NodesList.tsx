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

  // Combine the seed node (node_url) with peers (peer_details) to show all nodes
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

  const nodes = seedNode ? [seedNode, ...peerNodes] : peerNodes;
  const activeCount = nodes.filter(n => n.status === "active").length;

  const filteredNodes = nodes
    .filter(node => node.url.toLowerCase().includes(searchQuery.toLowerCase()))
    .sort((a, b) => a.url.localeCompare(b.url));

  if (compact) {
    return (
      <Card className="h-full flex flex-col border rounded-lg shadow-sm">
        <CardHeader className="pb-3 border-b">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base font-semibold">Nodes</CardTitle>
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="text-xs">
                {activeCount}/{nodes.length}
              </Badge>
            </div>
          </div>
          <div className="relative mt-3">
            <Search className="absolute left-2 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
            <Input
              placeholder="Search nodes..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-8 h-8 text-xs"
            />
          </div>
        </CardHeader>
        <ScrollArea className="flex-1">
          <div className="p-2">
            {filteredNodes.map((node) => {
              const isSelected = selectedNodeUrl === node.url;
              const isActive = node.status === "active";
              
              return (
                <div
                  key={node.url}
                  onClick={() => onSelectNode(node.url)}
                  className={`
                    group relative p-3 mb-2 rounded-lg cursor-pointer transition-all
                    ${isSelected 
                      ? "bg-primary/10 border-2 border-primary shadow-sm" 
                      : "bg-card hover:bg-muted/50 border-2 border-transparent"
                    }
                  `}
                >
                  {/* Status Indicator */}
                  <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-8 rounded-r-full transition-all"
                    style={{
                      backgroundColor: isActive ? "#84cc16" : node.status === "stale" ? "#eab308" : "#ef4444"
                    }}
                  />
                  
                  <div className="flex items-start gap-3 ml-2">
                    {/* Node Icon */}
                    <div className={`
                      mt-0.5 p-1.5 rounded-md transition-all
                      ${isActive ? "bg-green-500/10" : "bg-muted"}
                    `}>
                      <Server className={`h-3.5 w-3.5 ${isActive ? "text-green-600" : "text-muted-foreground"}`} />
                    </div>
                    
                    {/* Node Info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="text-sm font-medium truncate">
                          {node.url.split("//")[1] || node.url}
                        </span>
                        {node.isSeed && (
                          <Badge variant="secondary" className="text-xs px-1.5 py-0">
                            seed
                          </Badge>
                        )}
                        <Badge 
                          variant={isActive ? "outline" : "destructive"}
                          className={`text-xs px-1.5 py-0 ${isActive ? "text-green-600 border-green-600" : ""}`}
                        >
                          {node.status}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-3 text-xs text-muted-foreground">
                        <span className="flex items-center gap-1">
                          <Activity className="h-3 w-3" />
                          {node.heartbeat}
                        </span>
                        <span className="flex items-center gap-1">
                          <Clock className="h-3 w-3" />
                          {node.lag}ms
                        </span>
                      </div>
                    </div>
                    
                    {/* Arrow indicator on hover/select */}
                    <div className={`
                      transition-all
                      ${isSelected ? "opacity-100" : "opacity-0 group-hover:opacity-50"}
                    `}>
                      <ArrowRight className="h-4 w-4 text-muted-foreground" />
                    </div>
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
      </Card>
    );
  }

  return (
    <Card className="h-full flex flex-col">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Cluster Nodes ({nodes.length})</CardTitle>
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
            {nodes.length === 0 && (
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
