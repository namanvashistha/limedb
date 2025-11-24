# LimeDB Web Dashboard

A real-time monitoring dashboard for the LimeDB distributed key-value store, built with Next.js.

## Key Features

### 🎯 Equal Node Treatment
- **No Primary/Leader Concept**: All nodes in the cluster are treated equally
- **Alphabetical Sorting**: Nodes are displayed in alphabetical order by URL
- **No Self-Node Priority**: The node running the UI server has no special visual treatment
- **Uniform Display**: Each node card/row shows the same information format

### 🔍 Node Inspector
- **Dropdown Selector**: Choose any node from the cluster to inspect
- **Per-Node Gossip Map**: View the complete gossip state as seen by the selected node
- **Real-Time Metrics**: Heartbeat, cluster health, peer counts, lag, and more
- **Keys View**: See all keys stored on the selected node
- **Raw JSON**: Inspect the actual gossip payloads

### ⚡ Real-Time Updates
- **Auto-Refresh**: Updates every 2 seconds (configurable)
- **Heartbeat Animation**: Visual pulse when node heartbeat changes
- **Status Indicators**: Active (green) / Stale (yellow) / Dead (red)
- **Connection Status**: Live indicator in the header

## Getting Started

First, run the development server:

```bash
cd web
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

### Environment Variables

Create a `.env.local` file:

```bash
# Set the seed node URL (default: http://192.168.1.124:8484)
LIMEDB_SEED_URL=http://localhost:7001
```

## Architecture

### API Proxy
The dashboard includes a Next.js API proxy (`/api/proxy/[...path]`) that:
- Forwards requests to any LimeDB node
- Supports `?node=<url>` query parameter to target specific nodes
- Handles CORS and request/response transformation

### Data Flow
1. **Cluster Discovery**: Dashboard connects to seed node
2. **Gossip Enumeration**: Fetches gossip metrics to discover all peers
3. **Node Status**: Builds cluster view from gossip peer_details
4. **Per-Node Details**: When node selected, fetches that node's gossip/keys directly

## API Endpoints Used

### Cluster-Wide
- `GET /api/v1/cluster/gossip` - Gossip metrics
- `GET /api/v1/cluster/ring` - Hash ring state
- `GET /api/v1/keys` - List all keys on a node

### Per-Node (via `?node=<url>` param)
- `GET /api/v1/cluster/gossip?node=<url>` - Specific node's gossip view
- `GET /api/v1/keys?node=<url>` - Keys stored on specific node

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.
