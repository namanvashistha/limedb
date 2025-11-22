"use client";

import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { api } from "@/lib/api";
import { Loader2 } from "lucide-react";

interface HistoryItem {
  timestamp: string;
  action: "GET" | "SET" | "DEL";
  key: string;
  value?: string;
  result: any;
}

export function DataExplorer() {
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);
  const [history, setHistory] = useState<HistoryItem[]>([]);

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
      setHistory((prev) => [newItem, ...prev]);
    } catch (error) {
      setResponse({ error: "Request failed" });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card className="h-[500px] flex flex-col">
        <CardHeader>
          <CardTitle>Request</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 flex-1 flex flex-col">
          <div className="space-y-2">
            <Input
              placeholder="Key"
              value={key}
              onChange={(e) => setKey(e.target.value)}
            />
            <Input
              placeholder="Value (for SET)"
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
          </div>
          <div className="flex gap-2">
            <Button onClick={() => handleAction("GET")} disabled={loading}>GET</Button>
            <Button onClick={() => handleAction("SET")} variant="secondary" disabled={loading}>SET</Button>
            <Button onClick={() => handleAction("DEL")} variant="destructive" disabled={loading}>DEL</Button>
          </div>
          
          <div className="flex-1 rounded-md border bg-muted p-4 overflow-auto font-mono text-sm">
            {loading ? (
              <div className="flex items-center justify-center h-full">
                <Loader2 className="h-6 w-6 animate-spin" />
              </div>
            ) : (
              <pre>{JSON.stringify(response, null, 2)}</pre>
            )}
          </div>
        </CardContent>
      </Card>

      <Card className="h-[500px] flex flex-col">
        <CardHeader>
          <CardTitle>History</CardTitle>
        </CardHeader>
        <CardContent className="flex-1 p-0">
          <ScrollArea className="h-full px-4">
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
                    <span className={`font-bold ${
                      item.action === "GET" ? "text-blue-500" :
                      item.action === "SET" ? "text-green-500" : "text-red-500"
                    }`}>
                      {item.action}
                    </span>
                    <span className="text-xs text-muted-foreground">{item.timestamp}</span>
                  </div>
                  <div className="font-mono truncate">{item.key}</div>
                </div>
              ))}
              {history.length === 0 && (
                <div className="text-center text-muted-foreground py-8">
                  No history yet
                </div>
              )}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  );
}
