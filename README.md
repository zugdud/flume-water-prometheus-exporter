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

### For Raspberry Pi (Recommended)

**Pre-compiled binaries are available for Raspberry Pi:**

1. **Raspberry Pi 4 / Pi 400 / Pi Zero 2 W (64-bit):**
   ```bash
   wget https://github.com/flume-water-prometheus-exporter/releases/latest/download/flume-exporter-linux-arm64
   chmod +x flume-exporter-linux-arm64
   sudo mv flume-exporter-linux-arm64 /usr/local/bin/flume-exporter
   ```

2. **Raspberry Pi 3 / Pi 2 / Pi Zero (32-bit):**
   ```bash
   wget https://github.com/flume-water-prometheus-exporter/releases/latest/download/flume-exporter-linux-arm32
   chmod +x flume-exporter-linux-arm32
   sudo mv flume-exporter-linux-arm32 /usr/local/bin/flume-exporter
   ```

### From Source

```bash
git clone https://github.com/flume-water-prometheus-exporter.git
cd flume-water-prometheus-exporter

# For Raspberry Pi (cross-compile)
GOOS=linux GOARCH=arm64 go build -o flume-exporter-linux-arm64 .    # 64-bit Pi
GOOS=linux GOARCH=arm GOARM=7 go build -o flume-exporter-linux-arm32 .  # 32-bit Pi

# For local development
go build -o flume-exporter
```

### Using Go Install

```bash
go install github.com/flume-water-prometheus-exporter@latest
```

## Configuration

The exporter can be configured using command-line flags or environment variables.

### Environment Variables (Recommended)

```bash
export FLUME_CLIENT_ID="your_client_id"
export FLUME_CLIENT_SECRET="your_client_secret"  
export FLUME_USERNAME="your_username"
export FLUME_PASSWORD="your_password"
export LISTEN_ADDRESS=":8080"
export METRICS_PATH="/metrics"
```

### Command Line Flags

```bash
./flume-exporter \
  -client-id="your_client_id" \
  -client-secret="your_client_secret" \
  -username="your_username" \
  -password="your_password" \
  -listen-address=":8080" \
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
| `-listen-address` | `LISTEN_ADDRESS` | `:8080` | Address to listen on |
| `-metrics-path` | `METRICS_PATH` | `/metrics` | Path for metrics endpoint |
| `-scrape-interval` | - | `30s` | How often to scrape Flume API |
| `-timeout` | - | `10s` | HTTP request timeout |
| `-base-url` | `BASE_URL` | `https://api.flumewater.com` | Flume API base URL |

## Usage

### Quick Start (Raspberry Pi)

1. **Transfer files to your Raspberry Pi:**
   ```bash
   # On your development machine
   scp flume-exporter-linux-arm64 install-raspberry-pi.sh pi@your-pi-ip:~/
   
   # Or for 32-bit Pi
   scp flume-exporter-linux-arm32 install-raspberry-pi.sh pi@your-pi-ip:~/
   ```

2. **SSH to your Raspberry Pi and run the installer:**
   ```bash
   ssh pi@your-pi-ip
   chmod +x install-raspberry-pi.sh
   ./install-raspberry-pi.sh
   ```

3. **The installer will:**
   - Install the binary to `/usr/local/bin/flume-exporter`
   - Create configuration at `/etc/flume-exporter/config.env`
   - Install and start a systemd service
   - Prompt you for your Flume API credentials

### Manual Usage

1. **Start the exporter:**
   ```bash
   ./flume-exporter
   ```

2. **Verify it's working:**
   ```bash
   curl http://localhost:8080/metrics
   ```

3. **Add to Prometheus configuration:**
   ```yaml
   scrape_configs:
     - job_name: 'flume-water'
       static_configs:
         - targets: ['raspberry-pi-ip:8080']
       scrape_interval: 30s
   ```

### Systemd Service Management

```bash
# Check service status
sudo systemctl status flume-exporter

# View logs
sudo journalctl -u flume-exporter -f

# Restart service
sudo systemctl restart flume-exporter

# Stop service
sudo systemctl stop flume-exporter
```

## Metrics

The exporter provides the following metrics:

### Water Usage Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `flume_current_flow_rate_gallons_per_minute` | Gauge | Current water flow rate | `device_id`, `device_name`, `location` |
| `flume_hourly_water_usage_gallons` | Gauge | Water usage in the last hour | `device_id`, `device_name`, `location` |  
| `flume_daily_water_usage_gallons` | Gauge | Water usage today | `device_id`, `device_name`, `location` |
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
  -p 8080:8080 \
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
      - "8080:8080"
    environment:
      - FLUME_CLIENT_ID=your_client_id
      - FLUME_CLIENT_SECRET=your_client_secret
      - FLUME_USERNAME=your_username
      - FLUME_PASSWORD=your_password
    restart: unless-stopped
```

## API Reference

This exporter uses the [Flume Personal API](https://flumetech.readme.io/reference/introduction) which provides:

- **Authentication**: OAuth2 token-based authentication
- **Device Management**: List and query water monitoring devices
- **Usage Queries**: Historical and real-time water usage data
- **Flow Rate**: Current water flow measurements

### Supported Device Types

- **Bridge Devices (type=1)**: Network gateways (metadata only)
- **Sensor Devices (type=2)**: Water flow sensors (full metrics)

## Troubleshooting

### Common Issues

**Authentication Failed:**
- Verify your credentials are correct
- Check that your account is fully activated
- Ensure API access is enabled in the portal

**No Metrics:**
- Check that you have sensor devices (not just bridges)
- Verify devices are online and reporting data
- Check exporter logs for API errors

**High Memory Usage:**
- Reduce scrape interval if needed
- Monitor the number of devices and metrics

### Logging

The exporter logs to stdout. Increase verbosity by setting log level:

```bash
export LOG_LEVEL=debug
./flume-exporter
```

### Health Check

Visit `http://localhost:8080/` for exporter status and available metrics.

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