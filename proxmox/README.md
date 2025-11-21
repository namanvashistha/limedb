# 🔐 Secure Grafana Cloud Configuration

This directory contains scripts for securely managing Grafana Cloud credentials for LimeDB's OpenTelemetry integration.

## 🚀 Quick Setup

### Option 1: Use Configuration Helper (Recommended)

```bash
# Run the configuration helper
bash proxmox/configure-grafana.sh

# Follow the prompts to enter your Grafana Cloud credentials
# The script will save them securely to ~/.grafana/config
```

### Option 2: Manual Configuration

```bash
# Create the configuration directory
mkdir -p ~/.grafana

# Create the config file
cat > ~/.grafana/config << 'EOF'
GRAFANA_OTLP_ENDPOINT='https://otlp-gateway-prod-us-east-0.grafana.net/otlp'
GRAFANA_PROMETHEUS_ENDPOINT='https://prometheus-prod-10-prod-us-central-0.grafana.net/api/prom/push'
GRAFANA_CLOUD_TOKEN='your_base64_encoded_token_here'
EOF

# Secure the file
chmod 600 ~/.grafana/config
```

### Option 3: Environment Variables

```bash
# Export environment variables
export GRAFANA_OTLP_ENDPOINT='https://otlp-gateway-prod-us-east-0.grafana.net/otlp'
export GRAFANA_PROMETHEUS_ENDPOINT='https://prometheus-prod-10-prod-us-central-0.grafana.net/api/prom/push'
export GRAFANA_CLOUD_TOKEN='your_base64_encoded_token_here'

# Run the LXC script
bash proxmox/limedb_lxc.sh
```

## 📋 Getting Grafana Cloud Credentials

1. **Login to Grafana Cloud**: Go to your Grafana Cloud instance
2. **Navigate to Connections**: Connections → Add new connection → OpenTelemetry
3. **Copy OTLP Endpoint**: Copy the provided OTLP endpoint URL
4. **Generate Token**: Create an access token with metrics and traces permissions
5. **Encode Credentials**: Base64 encode your username:token
   ```bash
   echo -n "username:token" | base64
   ```

## 🏗️ Architecture

```
LimeDB (Proxmox LXC) → OTEL Collector → Grafana Cloud (LGTM)
                                      ├── Tempo (Traces)
                                      ├── Mimir (Metrics)
                                      └── Grafana (Dashboards)
```

## 🔒 Security

- Configuration stored in `~/.grafana/config` with 600 permissions (owner read/write only)
- No secrets hardcoded in scripts
- Base64 encoded credentials for HTTP Basic Auth
- Environment variables as fallback option

## 📁 File Structure

```
proxmox/
├── limedb_lxc.sh           # Main LXC setup script
├── configure-grafana.sh    # Grafana Cloud configuration helper
└── README.md              # This file

~/.grafana/
└── config                 # Secure credential storage
```

## 🚀 Usage

1. **Configure credentials** (one-time setup):
   ```bash
   bash proxmox/configure-grafana.sh
   ```

2. **Deploy LimeDB container**:
   ```bash
   bash proxmox/limedb_lxc.sh
   ```

3. **Verify installation**:
   ```bash
   # Enter the container
   pct enter <container-id>
   
   # Check services
   systemctl status limedb otelcol
   
   # View logs
   journalctl -u limedb -f
   journalctl -u otelcol -f
   ```

The LXC script will automatically detect and use your `~/.grafana/config` file, pre-configuring the OTEL Collector with your Grafana Cloud credentials!