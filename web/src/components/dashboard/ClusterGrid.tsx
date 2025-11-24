import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PeerDetail } from "@/lib/types";
import { Activity, Server, Wifi, WifiOff } from "lucide-react";

interface ClusterGridProps {
  peers: PeerDetail[];
  observerNode: string;
}

export function ClusterGrid({ peers, observerNode }: ClusterGridProps) {
  // Sort nodes alphabetically by URL to ensure consistent order
  const sortedPeers = [...peers].sort((a, b) => a.url.localeCompare(b.url));

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {sortedPeers.map((peer) => (
        <NodeCard key={peer.url} peer={peer} isObserver={peer.url === observerNode} />
      ))}
    </div>
  );
}

function NodeCard({ peer, isObserver }: { peer: PeerDetail; isObserver: boolean }) {
  const isActive = peer.status === "active";
  const isDead = peer.status === "dead";
  const isStale = peer.status === "stale";

  return (
    <Card className={`transition-all duration-300 ${isObserver ? 'border-primary shadow-md' : ''}`}>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium truncate" title={peer.url}>
          {peer.url}
        </CardTitle>
        {isActive ? (
          <Wifi className="h-4 w-4 text-green-500" />
        ) : (
          <WifiOff className="h-4 w-4 text-destructive" />
        )}
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-foreground">Status</span>
            <Badge
              variant={isActive ? "default" : "destructive"}
              className={
                isActive
                  ? "bg-green-500 hover:bg-green-600"
                  : isStale
                  ? "bg-yellow-500 hover:bg-yellow-600"
                  : ""
              }
            >
              {peer.status.toUpperCase()}
            </Badge>
          </div>
          
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-foreground">Heartbeat</span>
            <span className="font-mono text-sm">{peer.heartbeat}</span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-foreground">Lag</span>
            <div className="flex items-center gap-1">
              <Activity className="h-3 w-3 text-muted-foreground" />
              <span className={`font-mono text-sm ${peer.lag > 10 ? 'text-destructive' : 'text-muted-foreground'}`}>
                {peer.lag}
              </span>
            </div>
          </div>

          {isObserver && (
            <div className="mt-2 pt-2 border-t text-xs text-center text-primary font-medium">
              Current Observer
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
