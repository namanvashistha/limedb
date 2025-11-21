# 🔭 OpenTelemetry Integration for LimeDB

LimeDB includes comprehensive OpenTelemetry (OTEL) integration with traces, metrics, and logs collection, designed for production deployment in Grafana Cloud.

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   LimeDB Node   │    │  OTEL Collector  │    │  Grafana Cloud  │
│                 │────│                  │────│                 │
│ • HTTP Tracing  │    │ • OTLP Receiver  │    │ • Tempo (Traces)│
│ • HTTP Metrics  │    │ • Authentication │    │ • Mimir (Metrics)│
│ • Structured    │    │ • Batching       │    │ • Loki (Logs)   │
│   Logging       │    │ • Filtering      │    │ • Grafana UI    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## 🚀 Features Implemented

### ✅ HTTP Request Tracing
- **Automatic span creation** for all HTTP requests
- **Context propagation** across service boundaries  
- **Request attributes**: method, path, status code, duration
- **Custom trace headers** support for distributed tracing
- **FastHTTP integration** with custom header carrier

### ✅ HTTP Metrics Collection  
- **Request counter**: `http.server.request_count`
- **Request latency**: `http.server.request_duration` (histogram)
- **Labeled metrics**: method, route, status code
- **Real-time aggregation** with configurable intervals

### ✅ Service Discovery & Resource Detection
- **Automatic hostname detection** and labeling
- **Service identification**: limedb-node with version
- **Environment-based configuration** 
- **Container-aware resource detection**

## 🔧 Configuration

### Environment Variables
```bash
# OTEL Collector endpoint (required for telemetry)
OTEL_ENDPOINT="localhost:4317"

# Service identification
OTEL_SERVICE_NAME="limedb-node" 
OTEL_SERVICE_VERSION="1.0.0"

# Collector configuration
OTEL_EXPORTER_OTLP_ENDPOINT="https://otlp-gateway-prod-ap-south-1.grafana.net/otlp"
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic <base64-encoded-token>"
```

### Command Line Flags
```bash
# Start LimeDB with OTEL enabled
./limedb \
  -server.port 8484 \
  -node.url http://192.168.1.125:8484 \
  -otel.endpoint localhost:4317 \
  -node.peers http://192.168.1.126:8484
```

## 🐳 Proxmox LXC Deployment

### Quick Setup
```bash
# 1. Configure Grafana Cloud credentials
bash proxmox/configure-grafana.sh

# 2. Deploy LXC container with OTEL integration  
bash proxmox/limedb_lxc.sh
```

### Manual Configuration
```bash
# Create credentials file
mkdir -p ~/.grafana
cat > ~/.grafana/config << EOF
GRAFANA_OTLP_ENDPOINT='https://otlp-gateway-prod-us-east-0.grafana.net/otlp'
GRAFANA_CLOUD_USERNAME='123456'  # Instance ID
GRAFANA_CLOUD_PASSWORD='glsa_xxx'  # Access Token
EOF
chmod 600 ~/.grafana/config
```

## 📊 OTEL Collector Configuration

The Proxmox deployment includes a production-ready OTEL Collector with:

### 🔧 Advanced Processors
- **Resource Detection**: Automatic hostname and container detection
- **Transform Processor**: Clean unnecessary attributes from traces
- **Batch Processor**: Efficient data aggregation and transmission

### 🔐 Security Features  
- **BasicAuth Extension**: Secure Grafana Cloud authentication
- **TLS Encryption**: All data transmitted over HTTPS
- **Credential Isolation**: Secrets stored in `~/.grafana/config`

### 📈 Performance Optimization
- **Connectors**: Host metrics collection and forwarding
- **Memory Limiting**: Resource-constrained environment support
- **Retry Logic**: Automatic retry on transmission failures

## 🏃‍♂️ Running & Testing

### Start LimeDB with OTEL
```bash
# In development
go run cmd/server/main.go \f -server.port 8484 \
  -node.url http://localhost:8484 \
  -otel.endpoint localhost:4317

# In production (systemd service)
sudo systemctl start limedb
sudo systemctl status limedb
```

### Verify Telemetry
```bash
# Check health endpoint with tracing
curl http://localhost:8484/api/v1/health

# Generate test traffic with metrics
curl -X POST http://localhost:8484/api/v1/set \
  -H "Content-Type: application/json" \
  -d '{"key":"test","value":"otel-data"}'

curl http://localhost:8484/api/v1/get/test
```

### Monitor OTEL Collector
```bash
# Check collector status
sudo systemctl status otel-collector

# View collector logs
sudo journalctl -u otel-collector -f
```

## 📱 Grafana Cloud Dashboards

### Pre-built Queries

**HTTP Request Rate**
```promql
rate(http_server_request_count_total{service_name="limedb-node"}[5m])
```

**HTTP Request Latency P95**
```promql  
histogram_quantile(0.95, rate(http_server_request_duration_bucket{service_name="limedb-node"}[5m]))
```

**Trace Search (Tempo)**
```
{service.name="limedb-node"} | duration > 100ms
```

### Dashboard Panels
- **Request Volume**: Requests per second by endpoint
- **Error Rate**: 4xx/5xx responses percentage  
- **Latency Distribution**: P50, P95, P99 response times
- **Service Map**: Request flow between components
- **Trace Timeline**: Individual request traces

## 🐛 Debugging & Troubleshooting

### Common Issues

**OTEL Not Working**
```bash
# Check if endpoint is configured
echo $OTEL_ENDPOINT

# Verify collector connectivity  
curl -v http://localhost:4317/v1/traces
```

**Missing Traces**
```bash
# Check trace propagation headers
curl -H "traceparent: 00-0123456789abcdef0123456789abcdef-123456789abcdef0-01" \
  http://localhost:8484/api/v1/health
```

**High Memory Usage**
```bash
# Check collector resource usage
sudo systemctl status otel-collector
htop -p $(pgrep otel-collector)
```

### Log Analysis  
```bash
# LimeDB application logs
sudo journalctl -u limedb -f --since "1 hour ago"

# OTEL Collector logs
sudo journalctl -u otel-collector -f --since "1 hour ago"

# System resource monitoring
iostat -x 1
sar -u 1
```

## 🔗 Related Files

- `cmd/server/main.go` - OTEL initialization and service startup
- `internal/telemetry/telemetry.go` - OTEL SDK configuration  
- `internal/server/server.go` - HTTP tracing and metrics
- `internal/config/config.go` - Configuration management
- `proxmox/limedb_lxc.sh` - Automated Proxmox deployment
- `proxmox/configure-grafana.sh` - Credential configuration helper

## 📚 References

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [OTEL Collector Configuration](https://opentelemetry.io/docs/collector/configuration/)
- [Grafana Cloud OTEL Integration](https://grafana.com/docs/grafana-cloud/monitor-applications/application-observability/setup/opentelemetry/)
- [FastHTTP OTEL Instrumentation](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/github.com/valyala/fasthttp/otelhttp)