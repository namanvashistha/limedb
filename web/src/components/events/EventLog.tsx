"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Input } from "@/components/ui/input";
import { useState, useEffect } from "react";
import { Clock, Filter } from "lucide-react";

interface Event {
  id: string;
  timestamp: Date;
  type: "NODE_JOIN" | "NODE_LEAVE" | "NODE_UP" | "NODE_DOWN" | "KEY_SET" | "KEY_DELETE" | "INFO";
  message: string;
}

export function EventLog() {
  const [events, setEvents] = useState<Event[]>([]);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    // Simulate events - in real app, this would be a WebSocket or polling endpoint
    const addEvent = (type: Event["type"], message: string) => {
      const newEvent: Event = {
        id: Math.random().toString(36),
        timestamp: new Date(),
        type,
        message,
      };
      setEvents((prev) => [newEvent, ...prev].slice(0, 100)); // Keep last 100 events
    };

    // Simulate some initial events
    addEvent("INFO", "Dashboard initialized");
    
    // Random event generator for demo
    const interval = setInterval(() => {
      const eventTypes: Event["type"][] = ["NODE_UP", "KEY_SET", "INFO"];
      const type = eventTypes[Math.floor(Math.random() * eventTypes.length)];
      const messages: Record<Event["type"], string> = {
        NODE_JOIN: "Node joined the cluster",
        NODE_LEAVE: "Node left the cluster",
        NODE_UP: "Node http://192.168.0.124:8484 is healthy",
        NODE_DOWN: "Node is unreachable",
        KEY_SET: `Key "${Math.random().toString(36).slice(2, 8)}" updated`,
        KEY_DELETE: "Key deleted",
        INFO: "Cluster state refreshed",
      };
      addEvent(type, messages[type]);
    }, 5000);

    return () => clearInterval(interval);
  }, []);

  const filteredEvents = events.filter(
    (e) =>
      e.message.toLowerCase().includes(filter.toLowerCase()) ||
      e.type.toLowerCase().includes(filter.toLowerCase())
  );

  const getEventColor = (type: Event["type"]) => {
    switch (type) {
      case "NODE_JOIN":
      case "NODE_UP":
        return "bg-green-500";
      case "NODE_LEAVE":
      case "NODE_DOWN":
        return "bg-red-500";
      case "KEY_SET":
        return "bg-blue-500";
      case "KEY_DELETE":
        return "bg-orange-500";
      default:
        return "bg-gray-500";
    }
  };

  return (
    <Card className="h-[600px] flex flex-col">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Clock className="h-5 w-5" />
            Event Log
          </CardTitle>
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Filter events..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="w-64"
            />
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex-1 overflow-hidden p-0">
        <ScrollArea className="h-full px-6">
          <div className="space-y-2 pb-4">
            {filteredEvents.map((event) => (
              <div
                key={event.id}
                className="flex items-start gap-3 rounded-lg border p-3 hover:bg-muted/50 transition-colors"
              >
                <div className="flex-shrink-0 pt-1">
                  <div className={`h-2 w-2 rounded-full ${getEventColor(event.type)}`} />
                </div>
                <div className="flex-1 space-y-1">
                  <div className="flex items-center justify-between">
                    <Badge variant="outline" className="text-xs">
                      {event.type}
                    </Badge>
                    <span className="text-xs text-muted-foreground">
                      {event.timestamp.toLocaleTimeString()}
                    </span>
                  </div>
                  <p className="text-sm">{event.message}</p>
                </div>
              </div>
            ))}
            {filteredEvents.length === 0 && (
              <div className="text-center text-muted-foreground py-8">
                No events match your filter
              </div>
            )}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  );
}
