"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { ReplicaInfo } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Search, CheckCircle2, XCircle, Star, Server, Shield } from "lucide-react";

export function ReplicationInspector() {
    const [key, setKey] = useState("");
    const [loading, setLoading] = useState(false);
    const [info, setInfo] = useState<ReplicaInfo | null>(null);
    const [error, setError] = useState<string | null>(null);

    const lookup = async () => {
        if (!key.trim()) return;
        setLoading(true);
        setError(null);
        setInfo(null);
        try {
            const result = await api.getReplicaInfo(key.trim());
            setInfo(result);
        } catch (e: any) {
            setError(e.message || "Failed to fetch replica info");
        } finally {
            setLoading(false);
        }
    };

    const syncedCount = info?.replicas.filter((r) => r.has_value).length ?? 0;
    const quorumMet = info ? syncedCount >= info.quorum : false;

    return (
        <div className="space-y-4">
            {/* Search Bar */}
            <div className="flex gap-2">
                <Input
                    placeholder="Enter key to inspect replicas…"
                    value={key}
                    onChange={(e) => setKey(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && lookup()}
                    className="font-mono text-sm"
                />
                <Button onClick={lookup} disabled={loading || !key.trim()} size="sm">
                    <Search className="h-4 w-4 mr-1" />
                    {loading ? "Probing…" : "Inspect"}
                </Button>
            </div>

            {error && (
                <div className="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded-md">
                    {error}
                </div>
            )}

            {info && (
                <div className="space-y-3">
                    {/* Summary bar */}
                    <div className="flex items-center gap-3 flex-wrap">
                        <div className="flex items-center gap-1.5 text-sm">
                            <Shield className="h-4 w-4 text-muted-foreground" />
                            <span className="text-muted-foreground">RF={info.replication_factor}</span>
                        </div>
                        <div className="flex items-center gap-1.5 text-sm">
                            <span className="text-muted-foreground">Quorum={info.quorum}</span>
                        </div>
                        <Badge
                            variant={quorumMet ? "outline" : "destructive"}
                            className={quorumMet ? "text-green-600 border-green-600" : ""}
                        >
                            {syncedCount}/{info.replication_factor} synced
                            {quorumMet ? " ✓ QUORUM" : " ✗ BELOW QUORUM"}
                        </Badge>
                    </div>

                    {/* Replica nodes */}
                    <div className="space-y-2">
                        {info.replicas.map((replica) => (
                            <div
                                key={replica.node_url}
                                className={`
                  flex items-center justify-between px-3 py-2.5 rounded-lg border text-sm
                  ${replica.has_value
                                        ? "border-green-500/40 bg-green-500/5"
                                        : "border-red-500/30 bg-red-500/5"
                                    }
                `}
                            >
                                <div className="flex items-center gap-2 min-w-0">
                                    {/* Status icon */}
                                    {replica.has_value ? (
                                        <CheckCircle2 className="h-4 w-4 text-green-500 flex-shrink-0" />
                                    ) : (
                                        <XCircle className="h-4 w-4 text-red-500 flex-shrink-0" />
                                    )}

                                    {/* Node URL */}
                                    <span className="font-mono truncate text-xs">{replica.node_url}</span>
                                </div>

                                {/* Badges */}
                                <div className="flex items-center gap-1.5 flex-shrink-0 ml-2">
                                    {replica.is_primary && (
                                        <Badge variant="secondary" className="gap-1 text-xs px-1.5 py-0">
                                            <Star className="h-2.5 w-2.5" />
                                            primary
                                        </Badge>
                                    )}
                                    {replica.is_local && (
                                        <Badge variant="outline" className="text-xs px-1.5 py-0">
                                            <Server className="h-2.5 w-2.5 mr-1" />
                                            coordinator
                                        </Badge>
                                    )}
                                    <Badge
                                        variant={replica.has_value ? "outline" : "destructive"}
                                        className={`text-xs px-1.5 py-0 ${replica.has_value ? "text-green-600 border-green-600" : ""}`}
                                    >
                                        {replica.has_value ? "has key" : "missing"}
                                    </Badge>
                                </div>
                            </div>
                        ))}
                    </div>

                    {/* Explanation */}
                    <p className="text-[11px] text-muted-foreground">
                        <span className="font-medium">Primary</span> = first node clockwise on the ring.{" "}
                        <span className="font-medium">Missing</span> = node is alive but hasn&apos;t received the key yet (eventual consistency).
                    </p>
                </div>
            )}
        </div>
    );
}
