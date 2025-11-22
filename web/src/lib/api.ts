import { GossipMetrics, KeyValueResponse, NodeStatus, RingState } from "./types";

const BASE_URL = "/api/proxy";

export const api = {
  async getClusterStatus(): Promise<Record<string, NodeStatus>> {
    const res = await fetch(`${BASE_URL}/cluster/state`);
    if (!res.ok) throw new Error("Failed to fetch cluster status");
    return res.json();
  },

  async getRingState(): Promise<RingState> {
    const res = await fetch(`${BASE_URL}/cluster/ring`);
    if (!res.ok) throw new Error("Failed to fetch ring state");
    return res.json();
  },

  async getGossipMetrics(): Promise<GossipMetrics> {
    const res = await fetch(`${BASE_URL}/cluster/gossip`);
    if (!res.ok) throw new Error("Failed to fetch gossip metrics");
    return res.json();
  },

  async getKey(key: string): Promise<KeyValueResponse> {
    const res = await fetch(`${BASE_URL}/get/${key}`);
    const body = await res.text();
    return { status: res.status, body };
  },

  async setKey(key: string, value: string): Promise<KeyValueResponse> {
    const res = await fetch(`${BASE_URL}/set`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value }),
    });
    const body = await res.text();
    return { status: res.status, body };
  },

  async deleteKey(key: string): Promise<KeyValueResponse> {
    const res = await fetch(`${BASE_URL}/del/${key}`, {
      method: "DELETE",
    });
    const body = await res.text();
    return { status: res.status, body };
  },
};
