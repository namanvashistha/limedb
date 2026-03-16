"use client";

import { useState, useEffect, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { api } from "@/lib/api";
import { Database, Loader2, ChevronLeft, ChevronRight, Play, Trash2, Edit, RefreshCw, Layers, Zap, X } from "lucide-react";

import { faker } from "@faker-js/faker";

function loremKey() {
  return `${faker.word.adjective()}-${faker.animal.type()}-${faker.number.int({ min: 1, max: 9999 })}`;
}
function loremValue() {
  return faker.hacker.phrase();
}

interface KeyData {
  key: string;
  value: string;
  size: number;
  nodeUrl?: string;
}

type Operation = "GET" | "SET" | "DEL";

// Stable color per node URL based on index
const NODE_COLORS = [
  "bg-blue-500",
  "bg-green-500",
  "bg-purple-500",
  "bg-orange-500",
  "bg-pink-500",
  "bg-teal-500",
];

function NodeBadge({ nodeUrl, colorIndex }: { nodeUrl: string; colorIndex: number }) {
  const label = nodeUrl.replace(/^https?:\/\//, "").replace(/:8484$/, "");
  const color = NODE_COLORS[colorIndex % NODE_COLORS.length];
  return (
    <span className={`inline-flex items-center gap-1 text-xs font-mono text-white px-2 py-0.5 rounded-full ${color}`}>
      {label}
    </span>
  );
}

const ALL_NODES = "__all__";

export function EnhancedDataExplorer() {
  // Query form state
  const [operation, setOperation] = useState<Operation>("GET");
  const [queryKey, setQueryKey] = useState("");
  const [queryValue, setQueryValue] = useState("");
  const [queryResult, setQueryResult] = useState<any>(null);

  const [allKeys, setAllKeys] = useState<KeyData[]>([]);
  const [filteredKeys, setFilteredKeys] = useState<KeyData[]>([]);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [filterQuery, setFilterQuery] = useState("");

  // Node switcher
  const [selectedNode, setSelectedNode] = useState<string>(ALL_NODES);
  const [discoveredNodes, setDiscoveredNodes] = useState<string[]>([]);
  const [nodeColorMap, setNodeColorMap] = useState<Map<string, number>>(new Map());

  // Pagination
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 100;

  // Edit mode

  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");

  // Seed state
  const [showSeed, setShowSeed] = useState(false);
  const [seedCount, setSeedCount] = useState(100);
  const [seeding, setSeeding] = useState(false);
  const [seedProgress, setSeedProgress] = useState(0);

  // Discover nodes on mount
  useEffect(() => {
    api.discoverCluster().then((nodes) => {
      const sortedNodes = nodes.sort();
      setDiscoveredNodes(sortedNodes);
      const map = new Map<string, number>();
      sortedNodes.forEach((n, i) => map.set(n, i));
      setNodeColorMap(map);
    });
  }, []);

  const loadKeys = useCallback(async () => {
    setLoading(true);
    try {
      let keys: KeyData[] = [];

      if (selectedNode === ALL_NODES) {
        // Fan-out to all discovered nodes
        const result = await api.listAllKeys();
        keys = result.map((k) => ({
          key: k.key,
          value: k.value,
          size: k.size,
          nodeUrl: k.nodeUrl,
        }));
      } else {
        // Single node
        const result = await api.listKeysFromNode(selectedNode);
        keys = result.keys.map((k) => ({
          key: k.key,
          value: k.value,
          size: k.size,
          nodeUrl: selectedNode,
        }));
      }

      setAllKeys(keys);
      setFilteredKeys(keys);
      setCurrentPage(1);
    } catch (error) {
      console.error("Failed to load keys:", error);
      setAllKeys([]);
      setFilteredKeys([]);
    } finally {
      setLoading(false);
    }
  }, [selectedNode]);

  // Reload when selected node changes
  useEffect(() => {
    loadKeys();
  }, [loadKeys]);

  // Client-side filter
  useEffect(() => {
    if (!filterQuery.trim()) {
      setFilteredKeys(allKeys);
    } else {
      const term = filterQuery.toLowerCase();
      setFilteredKeys(allKeys.filter((k) => k.key.toLowerCase().includes(term)));
    }
    setCurrentPage(1);
  }, [filterQuery, allKeys]);

  const executeQuery = async () => {
    if (!queryKey.trim()) return;
    setExecuting(true);
    setQueryResult(null);
    try {
      if (operation === "GET") {
        const res = await api.getKey(queryKey);
        setQueryResult({ operation: "GET", key: queryKey, result: res });
      } else if (operation === "SET") {
        const res = await api.setKey(queryKey, queryValue);
        setQueryResult({ operation: "SET", key: queryKey, value: queryValue, result: res });
        loadKeys();
      } else if (operation === "DEL") {
        const res = await api.deleteKey(queryKey);
        setQueryResult({ operation: "DEL", key: queryKey, result: res });
        loadKeys();
      }
    } catch {
      setQueryResult({ error: "Query execution failed" });
    } finally {
      setExecuting(false);
    }
  };

  const handleGetKey = async (key: string) => {
    setExecuting(true);
    try {
      const res = await api.getKey(key);
      // Extract the actual value field from the JSON response
      let valueStr: string;
      try {
        const parsed = typeof res.body === "string" ? JSON.parse(res.body) : res.body;
        valueStr = parsed?.value ?? (typeof res.body === "string" ? res.body : JSON.stringify(res.body));
      } catch {
        valueStr = typeof res.body === "string" ? res.body : JSON.stringify(res.body);
      }
      setAllKeys((prev) =>
        prev.map((k) => (k.key === key ? { ...k, value: valueStr, size: valueStr.length } : k))
      );
    } catch (error) {
      console.error("Failed to get key:", error);
    } finally {
      setExecuting(false);
    }
  };

  const handleDeleteKey = async (key: string) => {
    if (!confirm(`Delete key "${key}"?`)) return;
    setExecuting(true);
    try {
      await api.deleteKey(key);
      setAllKeys((prev) => prev.filter((k) => k.key !== key));
    } catch (error) {
      console.error("Failed to delete key:", error);
    } finally {
      setExecuting(false);
    }
  };

  const handleSaveEdit = async (key: string) => {
    setExecuting(true);
    try {
      await api.setKey(key, editValue);
      setAllKeys((prev) =>
        prev.map((k) => (k.key === key ? { ...k, value: editValue, size: editValue.length } : k))
      );
      setEditingKey(null);
      setEditValue("");
    } catch (error) {
      console.error("Failed to update key:", error);
    } finally {
      setExecuting(false);
    }
  };

  const seedData = async () => {
    setSeeding(true);
    setSeedProgress(0);
    const total = seedCount;
    let done = 0;

    // Use all available nodes, or at least fallback to the seed URL
    const nodes = discoveredNodes.length > 0 ? discoveredNodes : [api.getSeedUrl()];

    // Generate batches of parallel requests to avoid ERR_INSUFFICIENT_RESOURCES
    const batchSize = 50;
    
    for (let i = 0; i < total; i += batchSize) {
      const currentBatchSize = Math.min(batchSize, total - i);
      const promises = Array.from({ length: currentBatchSize }, () => {
        const targetNode = nodes[Math.floor(Math.random() * nodes.length)];
        const key = loremKey() + "-" + Math.floor(Math.random() * 9999);
        const value = loremValue();
        
        return api.setKey(key, value, targetNode).finally(() => {
          done++;
          setSeedProgress(Math.round((done / total) * 100));
        });
      });

      // Wait for the current batch to complete before starting the next
      await Promise.allSettled(promises);
    }

    setSeeding(false);
    setShowSeed(false);
    setSeedProgress(0);
    loadKeys();
  };

  // Pagination
  const totalPages = Math.ceil(filteredKeys.length / pageSize);
  const startIndex = (currentPage - 1) * pageSize;
  const currentKeys = filteredKeys.slice(startIndex, startIndex + pageSize);

  return (
    <div className="space-y-4">
      {/* Structured Query Form */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            Query
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-2">
            {/* Operation selector */}
            <div className="flex rounded-md border overflow-hidden shrink-0">
              {(["GET", "SET", "DEL"] as Operation[]).map((op) => (
                <button
                  key={op}
                  onClick={() => setOperation(op)}
                  className={`px-3 py-2 text-xs font-mono font-semibold transition-colors
                    ${
                      operation === op
                        ? op === "GET"
                          ? "bg-blue-500 text-white"
                          : op === "SET"
                          ? "bg-green-500 text-white"
                          : "bg-red-500 text-white"
                        : "bg-background text-muted-foreground hover:text-foreground"
                    }`}
                >
                  {op}
                </button>
              ))}
            </div>

            {/* Key input */}
            <Input
              placeholder="key"
              value={queryKey}
              onChange={(e) => setQueryKey(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && operation !== "SET") executeQuery(); }}
              className="font-mono max-w-xs"
            />

            {/* Value input — only for SET */}
            {operation === "SET" && (
              <Input
                placeholder="value"
                value={queryValue}
                onChange={(e) => setQueryValue(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") executeQuery(); }}
                className="font-mono flex-1"
              />
            )}

            <Button onClick={executeQuery} disabled={executing || !queryKey.trim()}>
              <Play className="h-4 w-4 mr-2" />
              Run
            </Button>
          </div>

          {queryResult && (
            <div className="rounded-md border bg-muted p-4">
              <pre className="text-xs font-mono overflow-auto">
                {JSON.stringify(queryResult, null, 2)}
              </pre>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Keys Table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between flex-wrap gap-3">
            <CardTitle className="flex items-center gap-2">
              <Database className="h-5 w-5" />
              Documents
              <Badge variant="outline">{filteredKeys.length}</Badge>
            </CardTitle>

            <Button
              variant="outline"
              size="sm"
              onClick={() => { setShowSeed((v) => !v); setSeedProgress(0); }}
              className="text-xs"
            >
              <Zap className="h-3.5 w-3.5 mr-1.5" />
              Seed Data
            </Button>

            {/* Node Switcher + Search */}
            <div className="flex items-center gap-2 flex-wrap">
              <Input
                placeholder="Filter keys…"
                value={filterQuery}
                onChange={(e) => setFilterQuery(e.target.value)}
                className="font-mono h-8 w-40 text-xs"
              />
              <Layers className="h-4 w-4 text-muted-foreground shrink-0" />
              {/* All Nodes pill */}
              <button
                onClick={() => setSelectedNode(ALL_NODES)}
                className={`inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-full border transition-colors
                  ${selectedNode === ALL_NODES
                    ? "bg-foreground text-background border-foreground"
                    : "bg-background text-muted-foreground border-border hover:border-foreground hover:text-foreground"
                  }`}
              >
                All Nodes
              </button>

              {/* Per-node pills */}
              {discoveredNodes.map((nodeUrl) => {
                const colorIdx = nodeColorMap.get(nodeUrl) ?? 0;
                const color = NODE_COLORS[colorIdx % NODE_COLORS.length];
                const label = nodeUrl.replace(/^https?:\/\//, "").replace(/:8484$/, "");
                const isSelected = selectedNode === nodeUrl;
                return (
                  <button
                    key={nodeUrl}
                    onClick={() => setSelectedNode(nodeUrl)}
                    className={`inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-full border transition-colors
                      ${isSelected
                        ? `${color} text-white border-transparent`
                        : "bg-background text-muted-foreground border-border hover:border-foreground hover:text-foreground"
                      }`}
                  >
                    <span className={`w-1.5 h-1.5 rounded-full ${isSelected ? "bg-white" : color}`} />
                    {label}
                  </button>
                );
              })}

              <Button variant="outline" size="sm" onClick={loadKeys} disabled={loading}>
                <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
              </Button>
            </div>
          </div>
        </CardHeader>

        {/* Seed Data Panel */}
        {showSeed && (
          <div className="mx-6 mb-4 rounded-lg border bg-muted/50 p-4 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium">Generate random Lorem keys & values</p>
              <button onClick={() => setShowSeed(false)} className="text-muted-foreground hover:text-foreground">
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="flex items-center gap-3">
              <label className="text-xs text-muted-foreground whitespace-nowrap">Count</label>
              <Input
                type="number"
                min={1}
                max={1000}
                value={seedCount}
                onChange={(e) => setSeedCount(Math.max(1, Math.min(100000, Number(e.target.value))))}
                className="w-24 h-8 text-sm"
                disabled={seeding}
              />
              <Button
                size="sm"
                onClick={seedData}
                disabled={seeding}
                className="bg-amber-500 hover:bg-amber-600 text-white"
              >
                {seeding ? (
                  <><Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> {seedProgress}%</>
                ) : (
                  <><Zap className="h-3.5 w-3.5 mr-1.5" /> Generate</>
                )}
              </Button>
            </div>

            {/* Progress bar */}
            {seeding && (
              <div className="w-full bg-muted rounded-full h-1.5 overflow-hidden">
                <div
                  className="h-1.5 bg-amber-500 transition-all duration-300 rounded-full"
                  style={{ width: `${seedProgress}%` }}
                />
              </div>
            )}

            <p className="text-xs text-muted-foreground font-mono">
              e.g. key: <span className="text-foreground">ipsum-dolor-4821</span> → value: <span className="text-foreground">lorem sit amet consectetur adipiscing elit.</span>
            </p>
          </div>
        )}

        <CardContent>
          {loading ? (
            <div className="flex items-center justify-center h-64">
              <Loader2 className="h-8 w-8 animate-spin" />
            </div>
          ) : filteredKeys.length === 0 ? (
            <div className="text-center text-muted-foreground py-12">
              <Database className="h-12 w-12 mx-auto mb-4 opacity-20" />
              <p>No keys found</p>
              <p className="text-xs mt-2">Use the query field above to add data</p>
            </div>
          ) : (
            <>
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[280px]">Key</TableHead>
                      {selectedNode === ALL_NODES && (
                        <TableHead className="w-[120px]">Node</TableHead>
                      )}
                      <TableHead>Value</TableHead>
                      <TableHead className="w-[80px]">Size</TableHead>
                      <TableHead className="w-[150px] text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {currentKeys.map((item) => (
                      <TableRow key={item.key}>
                        <TableCell className="font-mono text-sm font-medium">
                          {item.key}
                        </TableCell>

                        {selectedNode === ALL_NODES && (
                          <TableCell>
                            {item.nodeUrl && (
                              <NodeBadge
                                nodeUrl={item.nodeUrl}
                                colorIndex={nodeColorMap.get(item.nodeUrl) ?? 0}
                              />
                            )}
                          </TableCell>
                        )}

                        <TableCell>
                          {editingKey === item.key ? (
                            <Textarea
                              value={editValue}
                              onChange={(e) => setEditValue(e.target.value)}
                              rows={3}
                              className="font-mono text-xs"
                            />
                          ) : (
                            <div className="font-mono text-xs text-muted-foreground truncate max-w-md">
                              {item.value}
                            </div>
                          )}
                        </TableCell>

                        <TableCell className="text-sm text-muted-foreground">
                          {item.size > 0 ? `${item.size}B` : "-"}
                        </TableCell>

                        <TableCell className="text-right">
                          <div className="flex gap-1 justify-end">
                            {editingKey === item.key ? (
                              <>
                                <Button variant="ghost" size="sm" onClick={() => handleSaveEdit(item.key)} disabled={executing}>
                                  Save
                                </Button>
                                <Button variant="ghost" size="sm" onClick={() => { setEditingKey(null); setEditValue(""); }}>
                                  Cancel
                                </Button>
                              </>
                            ) : (
                              <>
                                <Button variant="ghost" size="sm" onClick={() => handleGetKey(item.key)} disabled={executing}>
                                  View
                                </Button>
                                <Button variant="ghost" size="sm" onClick={() => { setEditingKey(item.key); setEditValue(item.value); }}>
                                  <Edit className="h-4 w-4" />
                                </Button>
                                <Button variant="ghost" size="sm" onClick={() => handleDeleteKey(item.key)} disabled={executing}>
                                  <Trash2 className="h-4 w-4 text-destructive" />
                                </Button>
                              </>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="flex items-center justify-between mt-4">
                  <div className="text-sm text-muted-foreground">
                    Showing {startIndex + 1}–{Math.min(startIndex + pageSize, filteredKeys.length)} of {filteredKeys.length}
                  </div>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => setCurrentPage((p) => Math.max(1, p - 1))} disabled={currentPage === 1}>
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                    <span className="text-sm flex items-center">
                      Page {currentPage} of {totalPages}
                    </span>
                    <Button variant="outline" size="sm" onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))} disabled={currentPage === totalPages}>
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
