import { useState, useEffect, useRef } from 'react';
import { HealthResponse } from './types';

export interface NodeHistoryPoint {
  timestamp: number;
  rps: number;
  latency_ms: number;
  gc_pause_ms: number;
  memory_mb: number;
}

export function useNodeHistory(healthData: HealthResponse | null, maxDataPoints: number = 30) {
  const [history, setHistory] = useState<NodeHistoryPoint[]>([]);
  // Use a ref to track the last seen timestamp so we don't record duplicate points
  // if the component re-renders without new data.
  const lastUpdateRef = useRef<number>(0);

  useEffect(() => {
    if (!healthData) return;

    const now = Date.now();
    // Only add a new point if at least 1 second has passed since the last one
    if (now - lastUpdateRef.current < 1000) return;
    
    lastUpdateRef.current = now;

    setHistory(prev => {
      const newPoint: NodeHistoryPoint = {
        timestamp: now,
        rps: healthData.requests_per_second || 0,
        latency_ms: healthData.average_latency_ms || 0,
        gc_pause_ms: healthData.gc_pause_ms || 0,
        memory_mb: healthData.memory_allocated_mb || 0,
      };

      const newHistory = [...prev, newPoint];
      // Keep only the last `maxDataPoints` items
      if (newHistory.length > maxDataPoints) {
        return newHistory.slice(newHistory.length - maxDataPoints);
      }
      return newHistory;
    });
  }, [healthData, maxDataPoints]);

  return history;
}
