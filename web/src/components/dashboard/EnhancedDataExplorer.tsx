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
import { Database, Loader2, ChevronLeft, ChevronRight, Play, Trash2, Edit } from "lucide-react";

interface KeyData {
  key: string;
  value: string;
  size: number;
}

export function EnhancedDataExplorer() {
  const [query, setQuery] = useState("");
  const [allKeys, setAllKeys] = useState<KeyData[]>([]);
  const [filteredKeys, setFilteredKeys] = useState<KeyData[]>([]);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [queryResult, setQueryResult] = useState<any>(null);
  
  // Pagination
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize] = useState(20);
  
  // Edit mode
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");

  const loadAllKeys = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.listKeys(currentPage, pageSize);
      const keys: KeyData[] = response.keys.map(k => ({
        key: k.key,
        value: k.value,
        size: k.size
      }));
      
      setAllKeys(keys);
      setFilteredKeys(keys);
    } catch (error) {
      console.error("Failed to load keys:", error);
      setAllKeys([]);
      setFilteredKeys([]);
    } finally {
      setLoading(false);
    }
  }, [currentPage, pageSize]);

  // Load all keys on mount and when page changes
  useEffect(() => {
    loadAllKeys();
  }, [loadAllKeys]);

  // Filter keys when query changes  
  useEffect(() => {
    if (!query.trim()) {
      setFilteredKeys(allKeys);
    } else {
      const searchTerm = query.toLowerCase();
      setFilteredKeys(
        allKeys.filter(k => k.key.toLowerCase().includes(searchTerm))
      );
    }
    setCurrentPage(1);
  }, [query, allKeys]);

  const executeQuery = async () => {
    const trimmed = query.trim();
    if (!trimmed) return;

    setExecuting(true);
    setQueryResult(null);

    try {
      const parts = trimmed.split(/\s+/);
      const command = parts[0].toUpperCase();
      const key = parts[1];

      if (command === "GET" && key) {
        const res = await api.getKey(key);
        setQueryResult({ command: "GET", key, result: res });
      } else if (command === "SET" && key) {
        const value = parts.slice(2).join(" ");
        const res = await api.setKey(key, value);
        setQueryResult({ command: "SET", key, value, result: res });
        loadAllKeys(); // Refresh keys
      } else if (command === "DEL" && key) {
        const res = await api.deleteKey(key);
        setQueryResult({ command: "DEL", key, result: res });
        loadAllKeys(); // Refresh keys
      } else {
        setQueryResult({ error: "Invalid command. Use: GET key, SET key value, or DEL key" });
      }
    } catch (error) {
      setQueryResult({ error: "Query execution failed" });
    } finally {
      setExecuting(false);
    }
  };

  const handleGetKey = async (key: string) => {
    setExecuting(true);
    try {
      const res = await api.getKey(key);
      const valueStr = typeof res.body === 'string' ? res.body : JSON.stringify(res.body);
      // Update the key in the list with actual value
      setAllKeys(prev => prev.map(k => 
        k.key === key ? { ...k, value: valueStr, size: valueStr.length } : k
      ));
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
      setAllKeys(prev => prev.filter(k => k.key !== key));
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
      setAllKeys(prev => prev.map(k => 
        k.key === key ? { ...k, value: editValue, size: editValue.length } : k
      ));
      setEditingKey(null);
      setEditValue("");
    } catch (error) {
      console.error("Failed to update key:", error);
    } finally {
      setExecuting(false);
    }
  };

  // Pagination
  const totalPages = Math.ceil(filteredKeys.length / pageSize);
  const startIndex = (currentPage - 1) * pageSize;
  const endIndex = startIndex + pageSize;
  const currentKeys = filteredKeys.slice(startIndex, endIndex);

  return (
    <div className="space-y-4">
      {/* Query Input */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            Query
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-2">
            <Input
              placeholder='Enter query: "GET key1" or "SET key1 value" or "DEL key1"'
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                  executeQuery();
                }
              }}
              className="font-mono"
            />
            <Button onClick={executeQuery} disabled={executing || !query.trim()}>
              <Play className="h-4 w-4 mr-2" />
              Execute
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

      {/* All Keys Table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <Database className="h-5 w-5" />
              Documents
              <Badge variant="outline">{filteredKeys.length}</Badge>
            </CardTitle>
            <Button variant="outline" size="sm" onClick={loadAllKeys} disabled={loading}>
              <Loader2 className={`h-4 w-4 mr-2 ${loading ? "animate-spin" : ""}`} />
              Refresh
            </Button>
          </div>
        </CardHeader>
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
                      <TableHead className="w-[300px]">Key</TableHead>
                      <TableHead>Value</TableHead>
                      <TableHead className="w-[100px]">Size</TableHead>
                      <TableHead className="w-[150px] text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {currentKeys.map((item) => (
                      <TableRow key={item.key}>
                        <TableCell className="font-mono text-sm font-medium">
                          {item.key}
                        </TableCell>
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
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleSaveEdit(item.key)}
                                  disabled={executing}
                                >
                                  Save
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => {
                                    setEditingKey(null);
                                    setEditValue("");
                                  }}
                                >
                                  Cancel
                                </Button>
                              </>
                            ) : (
                              <>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleGetKey(item.key)}
                                  disabled={executing}
                                >
                                  View
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => {
                                    setEditingKey(item.key);
                                    setEditValue(item.value);
                                  }}
                                >
                                  <Edit className="h-4 w-4" />
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleDeleteKey(item.key)}
                                  disabled={executing}
                                >
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
                    Showing {startIndex + 1}-{Math.min(endIndex, filteredKeys.length)} of {filteredKeys.length}
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                      disabled={currentPage === 1}
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                    <div className="flex items-center gap-1">
                      <span className="text-sm">
                        Page {currentPage} of {totalPages}
                      </span>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                      disabled={currentPage === totalPages}
                    >
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
