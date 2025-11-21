# LimeDB TUI

A Terminal User Interface for monitoring the LimeDB cluster.

## Running

Make sure you have `uv` installed.

**Important**: Host URLs are now mandatory and must be provided via command line arguments. No hardcoded defaults are used.

```bash
cd tui

# Connect to your cluster (REQUIRED)
uv run main.py --hosts http://192.168.1.125:8484,http://192.168.1.126:8484,http://192.168.1.127:8484

# Or connect to local test cluster
uv run main.py --hosts http://localhost:8484,http://localhost:8485,http://localhost:8486

# Show help
uv run main.py --help
```

## Features

- **Mandatory host configuration**: All host URLs must be provided via `--hosts` argument
- **Random load balancing**: Each query randomly selects from the provided host URLs  
- **Real-time monitoring**: Live status and metrics for all nodes
- **No hardcoded defaults**: Fully configurable cluster connections

## Examples

```bash
# Single node
uv run main.py --hosts http://192.168.1.125:8484

# Multiple nodes (recommended)
uv run main.py --hosts http://192.168.1.125:8484,http://192.168.1.126:8484,http://192.168.1.127:8484
```
