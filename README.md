# Flume Water Prometheus Exporter

A Prometheus exporter for [Flume](https://flumewater.com/) water monitoring devices. This exporter collects water usage metrics from the Flume API and exposes them in Prometheus format.

## Features

- 🌊 Real-time current flow rate monitoring
- 📊 Hourly and daily water usage tracking  
- 🏠 Multi-device support with location labels
- 🔒 Secure token-based authentication
- ⚡ Configurable scrape intervals
- 📈 Rich Prometheus metrics with device metadata
- 🛠️ Built-in health and performance monitoring

## Prerequisites

1. **Flume Account**: You need a Flume account with at least one device
2. **API Credentials**: Generate API credentials from the [Flume Customer Portal](https://portal.flumewater.com/)
   - Log in to the portal
   - Go to Settings page
   - Scroll down and click "Generate API Client"
   - Save your Client ID and Client Secret

## Installation

### Prerequisites

- **Go 1.21+** installed on your system
- **Flume Account** with at least one device
- **API Credentials** from the [Flume Customer Portal](https://portal.flumewater.com/)

### Install Go

**On Linux/macOS:**
```bash
# Download and install Go
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version
```

**On Windows:**
- Download from [https://go.dev/dl/](https://go.dev/dl/)
- Run the installer and follow the prompts
- Verify installation: `go version`

**On Raspberry Pi (ARM64):**
```bash
# Download ARM64 version
wget https://go.dev/dl/go1.21.6.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-arm64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version
```

### Build the Exporter

```bash
# Clone the repository
git clone https://github.com/yourusername/flume-water-prometheus-exporter.git
cd flume-water-prometheus-exporter

# Build the exporter
CGO_ENABLED=0 go build

# Make it executable (Linux/macOS)
chmod +x flume-exporter
```

The binary will be created as `flume-exporter` (or `flume-exporter.exe` on Windows).

## Configuration

The exporter can be configured using command-line flags or environment variables.

### Environment Variables (Recommended)

```bash
export FLUME_CLIENT_ID="your_client_id"
export FLUME_CLIENT_SECRET="your_client_secret"  
export FLUME_USERNAME="your_username"
export FLUME_PASSWORD="your_password"
export LISTEN_ADDRESS=":9193"
export METRICS_PATH="/metrics"
```

### Command Line Flags

```bash
./flume-exporter \
  -client-id="your_client_id" \
  -client-secret="your_client_secret" \
  -username="your_username" \
  -password="your_password" \
  -listen-address=":9193" \
  -metrics-path="/metrics" \
  -scrape-interval="30s" \
  -timeout="10s"
```

### Configuration Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `-client-id` | `FLUME_CLIENT_ID` | *required* | Flume API client ID |
| `-client-secret` | `FLUME_CLIENT_SECRET` | *required* | Flume API client secret |
| `-username` | `FLUME_USERNAME` | *required* | Flume account username |
| `-password` | `FLUME_PASSWORD` | *required* | Flume account password |
| `-listen-address` | `LISTEN_ADDRESS` | `:9193` | Address to listen on |
| `-metrics-path` | `METRICS_PATH` | `/metrics` | Path for metrics endpoint |
| `-scrape-interval` | `SCRAPE_INTERVAL` | `30s` | How often to scrape Flume API |
| `-timeout` | `TIMEOUT` | `10s` | HTTP request timeout |
| `-base-url` | `BASE_URL` | `https://api.flumewater.com` | Flume API base URL |
| `-api-min-interval` | `API_MIN_INTERVAL` | `30s` | Minimum interval between Flume API requests (120 requests/hour limit) |

## Usage

### Quick Start

1. **Build the exporter:**
   ```bash
   CGO_ENABLED=0 go build
   ```

2. **Set environment variables:**
   ```bash
   export FLUME_CLIENT_ID="your_client_id"
   export FLUME_CLIENT_SECRET="your_client_secret"
   export FLUME_USERNAME="your_username"
   export FLUME_PASSWORD="your_password"
   ```

3. **Run the exporter:**
   ```bash
   ./flume-exporter
   ```

4. **Verify it's working:**
   ```bash
   curl http://localhost:9193/metrics
   ```

**Note**: To run the exporter as a background service, you can use tools like `nohup`, `screen`, `tmux`, or create your own systemd service file. The exporter will continue running until stopped with Ctrl+C or the process is terminated.

**Add to Prometheus configuration:**
```yaml
scrape_configs:
  - job_name: 'flume-water'
    static_configs:
      - targets: ['localhost:9193']
    scrape_interval: 30s
```

### Optional: Setup as Systemd Service

If you want to run the exporter as a background service that starts automatically on boot:

1. **Create the service file:**
   ```bash
   sudo nano /etc/systemd/system/flume-exporter.service
   ```

2. **Add the following content:**
   ```ini
   [Unit]
   Description=Flume Water Prometheus Exporter
   After=network.target
   
   [Service]
   Type=simple
   User=flume-exporter
   Group=flume-exporter
   WorkingDirectory=/opt/flume-exporter
   ExecStart=/opt/flume-exporter/flume-exporter
   Restart=always
   RestartSec=10
   EnvironmentFile=/opt/flume-exporter/config.env
   
   [Install]
   WantedBy=multi-user.target
   ```

3. **Create service user and directory:**
   ```bash
   sudo useradd -r -s /bin/false flume-exporter
   sudo mkdir -p /opt/flume-exporter
   sudo cp flume-exporter /opt/flume-exporter/
   sudo chown -R flume-exporter:flume-exporter /opt/flume-exporter
   ```

4. **Create environment file:**
   ```bash
   sudo nano /opt/flume-exporter/config.env
   ```
   
   Add your Flume credentials:
   ```bash
   FLUME_CLIENT_ID=your_client_id
   FLUME_CLIENT_SECRET=your_client_secret
   FLUME_USERNAME=your_username
   FLUME_PASSWORD=your_password
   LISTEN_ADDRESS=:9193
   SCRAPE_INTERVAL=30s
   API_MIN_INTERVAL=30s
   ```

5. **Set permissions and enable service:**
   ```bash
   sudo chmod 600 /opt/flume-exporter/config.env
   sudo systemctl daemon-reload
   sudo systemctl enable flume-exporter
   sudo systemctl start flume-exporter
   ```

6. **Verify service is running:**
   ```bash
   sudo systemctl status flume-exporter
   sudo journalctl -u flume-exporter -f
   ```

**Service management commands:**
```bash
sudo systemctl start flume-exporter    # Start service
sudo systemctl stop flume-exporter     # Stop service
sudo systemctl restart flume-exporter  # Restart service
sudo systemctl status flume-exporter   # Check status
sudo journalctl -u flume-exporter -f  # View logs
```

### Configuration

The exporter can be configured using command-line flags or environment variables.

### Environment Variables (Recommended)

```bash
export FLUME_CLIENT_ID="your_client_id"
export FLUME_CLIENT_SECRET="your_client_secret"  
export FLUME_USERNAME="your_username"
export FLUME_PASSWORD="your_password"
export LISTEN_ADDRESS=":9193"
export METRICS_PATH="/metrics"
```

## Metrics

The exporter provides the following metrics:

### Water Usage Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `flume_current_flow_rate_gallons_per_minute` | Gauge | Current water flow rate (direct from API) | `device_id`, `device_name`, `location` |
| `flume_hourly_water_usage_gallons` | Gauge | Water usage in the last hour | `device_id`, `device_name`, `location` |  
| `flume_daily_water_usage_gallons` | Gauge | Water usage today | `device_id`, `device_name`, `location` |
| `flume_daily_total_water_usage_gallons` | Gauge | Daily total water usage for each day over time period | `device_id`, `device_name`, `location`, `date` |
| `flume_total_water_usage_gallons` | Gauge | Total usage for time period | `device_id`, `device_name`, `location`, `bucket` |

### Device Information

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `flume_device_info` | Gauge | Device metadata (always 1) | `device_id`, `device_name`, `location`, `device_type` |

### Exporter Health Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `flume_exporter_scrape_duration_seconds` | Gauge | Time spent scraping API | `endpoint` |
| `flume_exporter_scrape_success` | Gauge | Whether last scrape succeeded (1/0) | `endpoint` |
| `flume_exporter_last_scrape_timestamp_seconds` | Gauge | Unix timestamp of last scrape | `endpoint` |

## Example Queries

### Grafana Dashboard Queries

**Current Flow Rate:**
```promql
flume_current_flow_rate_gallons_per_minute
```

**Daily Water Usage:**
```promql
flume_daily_water_usage_gallons
```

**Daily Total Water Usage (30-day history):**
```promql
flume_daily_total_water_usage_gallons
```

**Average Hourly Usage (24h):**
```promql
avg_over_time(flume_hourly_water_usage_gallons[24h])
```

**Water Usage Rate of Change:**
```promql
rate(flume_total_water_usage_gallons[5m]) * 60
```

## Docker

### Build Docker Image

```bash
docker build -t flume-exporter .
```

### Run with Docker

```bash
docker run -d \
  --name flume-exporter \
  -p 9193:9193 \
  -e FLUME_CLIENT_ID="your_client_id" \
  -e FLUME_CLIENT_SECRET="your_client_secret" \
  -e FLUME_USERNAME="your_username" \
  -e FLUME_PASSWORD="your_password" \
  flume-exporter
```

### Docker Compose

```yaml
version: '3.8'
services:
  flume-exporter:
    build: .
    ports:
      - "9193:9193"
    environment:
      - FLUME_CLIENT_ID=your_client_id
      - FLUME_CLIENT_SECRET=your_client_secret
      - FLUME_USERNAME=your_username
      - FLUME_PASSWORD=your_password
    restart: unless-stopped
```

## Rate Limiting

The Flume Water API has a rate limit of **120 requests per hour** for personal clients. This exporter automatically respects this limit by:

- **Default Configuration**: Limits API requests to a minimum of 30 seconds apart (120 requests/hour)
- **Configurable**: You can adjust the rate limiting via the `API_MIN_INTERVAL` environment variable or `-api-min-interval` flag
- **Per-Request Limiting**: Each API call (devices, flow rate, water usage) is individually rate-limited
- **Automatic Throttling**: The exporter will automatically wait between requests to stay within limits

**Example Rate Limiting Configuration:**
```bash
# Conservative: 60 seconds between requests (60 requests/hour)
export API_MIN_INTERVAL=60s

# Default: 30 seconds between requests (120 requests/hour)  
export API_MIN_INTERVAL=30s

# Aggressive: 20 seconds between requests (180 requests/hour - may exceed limits)
export API_MIN_INTERVAL=20s
```

**Note**: With the default 30-second scrape interval and 30-second API rate limit, the exporter will make approximately 3-4 API calls per device per scrape cycle. For most users with 1-2 devices, this keeps you well within the 120 requests/hour limit.

## API Reference

This exporter uses the [Flume Personal API](https://flumetech.readme.io/reference/introduction) which provides:

- **Authentication**: OAuth2 token-based authentication
- **Device Management**: List and query water monitoring devices
- **Usage Queries**: Historical and real-time water usage data
- **Flow Rate**: Calculated from recent water usage data (last 5 minutes)

### Supported Device Types

- **Bridge Devices (type=1)**: Network gateways (metadata only)
- **Sensor Devices (type=2)**: Water flow sensors (full metrics)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Disclaimer

This is an unofficial exporter for Flume devices. It is not affiliated with or endorsed by Flume, Inc.

## Related Projects

- [Prometheus](https://prometheus.io/) - Monitoring system and time series database
- [Grafana](https://grafana.com/) - Visualization and alerting platform
- [Node Exporter](https://github.com/prometheus/node_exporter) - Hardware and OS metrics exporter

---

🌊 **Happy Water Monitoring!** 🌊