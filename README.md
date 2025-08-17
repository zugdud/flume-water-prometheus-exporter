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

1. **Raspberry Pi 5 / Pi 4 / Pi 400 / Pi Zero 2 W (64-bit):**
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

# For cross-compilation (from development machine)
GOOS=linux GOARCH=arm64 go build -o flume-exporter-linux-arm64 .    # 64-bit Pi
GOOS=linux GOARCH=arm GOARM=7 go build -o flume-exporter-linux-arm32 .  # 32-bit Pi

# For building directly on Raspberry Pi 5 with Go 1.23+
go build -o flume-exporter .

# Using Makefile for optimized builds
make build-pi5          # Optimized for Pi 5
make build-linux-arm64  # General 64-bit ARM
make build-all          # All platforms
```

### Building on Raspberry Pi 5

**Requirements:**
- Raspberry Pi OS (64-bit) 
- Go 1.23+ installed

**Install Go 1.23 on Pi 5:**
```bash
# Remove old Go version if present
sudo rm -rf /usr/local/go

# Download and install Go 1.23 for ARM64
wget https://go.dev/dl/go1.23.4.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.23.4.linux-arm64.tar.gz

# Add to PATH (add to ~/.bashrc for permanent)
export PATH=$PATH:/usr/local/go/bin
```

**Build the exporter:**
```bash
git clone https://github.com/flume-water-prometheus-exporter.git
cd flume-water-prometheus-exporter

# Simple build script for Pi 5 (recommended)
chmod +x build-pi5.sh
./build-pi5.sh

# Or manual build
go mod tidy
go build -o flume-exporter .

# Or use Makefile targets
make build-pi5          # Pi 5 optimized
make build-linux-arm64  # Generic ARM64
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
| `-scrape-interval` | `SCRAPE_INTERVAL` | `30s` | How often to scrape Flume API |
| `-timeout` | `TIMEOUT` | `10s` | HTTP request timeout |
| `-base-url` | `BASE_URL` | `https://api.flumewater.com` | Flume API base URL |
| `-api-min-interval` | `API_MIN_INTERVAL` | `30s` | Minimum interval between Flume API requests (120 requests/hour limit) |

## Usage

### Quick Start (Raspberry Pi)

1. **Transfer files to your Raspberry Pi:**
   ```bash
   # For Raspberry Pi 5 (optimized build)
   scp flume-exporter-pi5-arm64 install-raspberry-pi.sh pi@your-pi-ip:~/
   
   # For other Pi models
   scp flume-exporter-linux-arm64 install-raspberry-pi.sh pi@your-pi-ip:~/   # 64-bit
   scp flume-exporter-linux-arm32 install-raspberry-pi.sh pi@your-pi-ip:~/   # 32-bit
   
   # Or build directly on Pi 5
   scp build-pi5.sh install-raspberry-pi.sh *.go go.mod go.sum Makefile pi@your-pi-ip:~/
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

## Raspberry Pi Installation

### Prerequisites

- Raspberry Pi 5 running Raspberry Pi OS (or compatible Linux distribution)
- Go 1.21+ installed (for building from source)
- Git installed

### Quick Installation (Recommended)

Use the provided installation script for automatic setup:

```bash
# Make script executable and run
chmod +x install-raspberry-pi.sh
./install-raspberry-pi.sh
```

The script will:
- Build the exporter for ARM64
- Install it to `/usr/local/bin/`
- Create configuration directory and files
- Install and enable the systemd service
- Start the service automatically

### Building and Installing

1. **Clone the repository:**
   ```bash
   git clone https://github.com/yourusername/flume-water-prometheus-exporter.git
   cd flume-water-prometheus-exporter
   ```

2. **Build for ARM64:**
   ```bash
   GOOS=linux GOARCH=arm64 go build -o flume-exporter
   ```

3. **Install the binary:**
   ```bash
   sudo cp flume-exporter /usr/local/bin/
   sudo chmod +x /usr/local/bin/flume-exporter
   ```

4. **Create configuration directory:**
   ```bash
   sudo mkdir -p /etc/flume-exporter
   ```

5. **Create environment file:**
   ```bash
   sudo nano /etc/flume-exporter/config.env
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

6. **Set proper permissions:**
   ```bash
   sudo chown root:root /etc/flume-exporter/config.env
   sudo chmod 600 /etc/flume-exporter/config.env
   ```

7. **Install systemd service:**
   ```bash
   sudo cp flume-exporter.service /etc/systemd/system/
   sudo systemctl daemon-reload
   ```

8. **Enable and start the service:**
   ```bash
   sudo systemctl enable flume-exporter
   sudo systemctl start flume-exporter
   ```

### Service Management

**Check service status:**
```bash
sudo systemctl status flume-exporter
```

**View logs:**
```bash
sudo journalctl -u flume-exporter -f
```

**Restart service:**
```bash
sudo systemctl restart flume-exporter
```

**Stop service:**
```bash
sudo systemctl stop flume-exporter
```

**Disable service (remove from startup):**
```bash
sudo systemctl disable flume-exporter
```

### Verification

1. **Check if service is running:**
   ```bash
   sudo systemctl is-active flume-exporter
   ```

2. **Test metrics endpoint:**
   ```bash
   curl http://localhost:9193/metrics
   ```

3. **Check service logs for any errors:**
   ```bash
   sudo journalctl -u flume-exporter --no-pager -l
   ```

### Troubleshooting

**Service won't start:**
- Check configuration file permissions: `ls -la /etc/flume-exporter/`
- Verify environment file syntax: `sudo cat /etc/flume-exporter/config.env`
- Check systemd logs: `sudo journalctl -u flume-exporter --no-pager -l`

**Permission denied errors:**
- Ensure binary is executable: `ls -la /usr/local/bin/flume-exporter`
- Check service user permissions in `flume-exporter.service`

**Port already in use:**
- Change `LISTEN_ADDRESS` in config.env to use a different port
- Check what's using the port: `sudo netstat -tlnp | grep :9193`

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

**Flow Rate Issues:**
- The exporter now uses the direct flow rate endpoint `/users/{user_id}/devices/{device_id}/query/active`
- This provides real-time flow rate data directly from the Flume API
- The exporter automatically fetches the user ID from the `/me` endpoint first (which returns the user ID in the `id` field)
- No more complex calculations from water usage data

**Rate Limiting Issues:**
- If you see "429 Too Many Requests" errors, increase your `API_MIN_INTERVAL`
- The default 30-second interval ensures you stay within the 120 requests/hour limit
- Monitor your API usage and adjust the rate limiting if needed

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