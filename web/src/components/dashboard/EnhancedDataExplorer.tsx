"use client";

import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { api } from "@/lib/api";
import { Database, Loader2, Copy, Trash2, Plus } from "lucide-react";

interface HistoryItem {
  timestamp: string;
  action: "GET" | "SET" | "DEL";
  key: string;
  value?: string;
  result: any;
}

export function EnhancedDataExplorer() {
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  
  // Bulk operations
  const [bulkKeys, setBulkKeys] = useState("");
  const [bulkValue, setBulkValue] = useState("");

  const handleAction = async (action: "GET" | "SET" | "DEL") => {
    if (!key) return;

    setLoading(true);
    setResponse(null);

    try {
      let res;
      if (action === "GET") {
        res = await api.getKey(key);
      } else if (action === "SET") {
        res = await api.setKey(key, value);
      } else {
        res = await api.deleteKey(key);
      }

      // Try to parse JSON body
      let parsedBody = res.body;
      try {
        if (typeof res.body === "string") {
          parsedBody = JSON.parse(res.body);
        }
      } catch {}

      const finalRes = { ...res, body: parsedBody };
      setResponse(finalRes);

      // Add to history
      const newItem: HistoryItem = {
        timestamp: new Date().toLocaleTimeString(),
        action,
        key,
        value: action === "SET" ? value : undefined,
        result: finalRes,
      };
      setHistory((prev) => [newItem, ...prev].slice(0, 50));
    } catch (error) {
      setResponse({ error: "Request failed" });
    } finally {
      setLoading(false);
    }
  };

  const handleBulkSet = async () => {
    const keys = bulkKeys.split("\n").filter((k) => k.trim());
    if (keys.length === 0 || !bulkValue) return;

    setLoading(true);
    const results = [];

    for (const k of keys) {
      try {
        const res = await api.setKey(k.trim(), bulkValue);
        results.push({ key: k.trim(), status: "success", result: res });
      } catch (error) {
        results.push({ key: k.trim(), status: "error", error });
      }
    }

    setResponse({ bulk: true, results });
    setLoading(false);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {/* Request Panel */}
      <Card className="h-[600px] flex flex-col">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            Data Operations
          </CardTitle>
        </CardHeader>
        <CardContent className="flex-1 flex flex-col space-y-4">
          <Tabs defaultValue="single" className="flex-1 flex flex-col">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="single">Single Operation</TabsTrigger>
              <TabsTrigger value="bulk">Bulk Operations</TabsTrigger>
            </TabsList>
            
            <TabsContent value="single" className="flex-1 flex flex-col space-y-4">
              <div className="space-y-2">
                <Input
                  placeholder="Key"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                />
                <Textarea
                  placeholder="Value (for SET operations)"
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  rows={4}
                />
              </div>
              <div className="flex gap-2">
                <Button onClick={() => handleAction("GET")} disabled={loading} className="flex-1">
                  GET
                </Button>
                <Button onClick={() => handleAction("SET")} variant="secondary" disabled={loading} className="flex-1">
                  SET
                </Button>
                <Button onClick={() => handleAction("DEL")} variant="destructive" disabled={loading} className="flex-1">
                  DELETE
                </Button>
              </div>
              
              <div className="flex-1 flex flex-col">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium">Response</span>
                  {response && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => copyToClipboard(JSON.stringify(response, null, 2))}
                    >
                      <Copy className="h-4 w-4" />
                    </Button>
                  )}
                </div>
                <div className="flex-1 rounded-md border bg-muted p-4 overflow-auto font-mono text-sm">
                  {loading ? (
                    <div className="flex items-center justify-center h-full">
                      <Loader2 className="h-6 w-6 animate-spin" />
                    </div>
                  ) : (
                    <pre className="text-xs">{JSON.stringify(response, null, 2)}</pre>
                  )}
                </div>
              </div>
            </TabsContent>
            
            <TabsContent value="bulk" className="flex-1 flex flex-col space-y-4">
              <div className="space-y-2">
                <Textarea
                  placeholder="Keys (one per line)"
                  value={bulkKeys}
                  onChange={(e) => setBulkKeys(e.target.value)}
                  rows={6}
                />
                <Input
                  placeholder="Value to set for all keys"
                  value={bulkValue}
                  onChange={(e) => setBulkValue(e.target.value)}
                />
              </div>
              <Button onClick={handleBulkSet} disabled={loading} className="w-full">
                <Plus className="h-4 w-4 mr-2" />
                Bulk SET
              </Button>
              
              <div className="flex-1 rounded-md border bg-muted p-4 overflow-auto font-mono text-sm">
                {loading ? (
                  <div className="flex items-center justify-center h-full">
                    <Loader2 className="h-6 w-6 animate-spin" />
                  </div>
                ) : (
                  <pre className="text-xs">{JSON.stringify(response, null, 2)}</pre>
                )}
              </div>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {/* History Panel */}
      <Card className="h-[600px] flex flex-col">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>History</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => setHistory([])}>
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </CardHeader>
        <CardContent className="flex-1 overflow-hidden p-0">
          <ScrollArea className="h-full px-6">
            <div className="space-y-2 pb-4">
              {history.map((item, i) => (
                <div
                  key={i}
                  className="flex flex-col gap-1 rounded-lg border p-3 text-sm hover:bg-muted/50 cursor-pointer transition-colors"
                  onClick={() => {
                    setKey(item.key);
                    setValue(item.value || "");
                    setResponse(item.result);
                  }}
                >
                  <div className="flex items-center justify-between">
                    <span
                      className={`font-bold ${
                        item.action === "GET"
                          ? "text-blue-500"
                          : item.action === "SET"
                          ? "text-green-500"
                          : "text-red-500"
                      }`}
                    >
                      {item.action}
                    </span>
                    <span className="text-xs text-muted-foreground">{item.timestamp}</span>
                  </div>
                  <div className="font-mono truncate">{item.key}</div>
                  {item.value && (
                    <div className="text-xs text-muted-foreground truncate">{item.value}</div>
                  )}
                </div>
              ))}
              {history.length === 0 && (
                <div className="text-center text-muted-foreground py-8">No history yet</div>
              )}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  );
}
