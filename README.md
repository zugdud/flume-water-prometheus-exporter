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
- 🎯 Optional device filtering by device ID

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

# Optional: Filter specific devices (comma-separated)
# If not specified, data from all devices will be collected
export DEVICE_IDS="6899913485570306485,6906448283393854879"
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
  -timeout="10s" \
  -device-ids="6899913485570306485,6906448283393854879"
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
| `SCRAPE_INTERVAL` | `30s` | How often to collect metrics from Flume API (auto-optimized based on device count) |
| `-timeout` | `TIMEOUT` | `10s` | HTTP request timeout |
| `-base-url` | `BASE_URL` | `https://api.flumewater.com` | Flume API base URL |
| `-api-min-interval` | `API_MIN_INTERVAL` | `30s` | Minimum interval between Flume API requests (120 requests/hour limit) |
| `-device-ids` | `DEVICE_IDS` | *none* | Comma-separated list of device IDs to collect data from (if not specified, all devices are collected) |

## Device Filtering

The exporter supports filtering which devices to collect data from using the `DEVICE_IDS` configuration option.

### How It Works

- **No Filter**: If `DEVICE_IDS` is not specified, the exporter will collect data from all devices in your Flume account
- **With Filter**: If `DEVICE_IDS` is specified, only the listed devices will be processed

### Configuration Examples

**Environment Variable:**
```bash
# Filter to specific devices
export DEVICE_IDS="6899913485570306485,6906448283393854879"

# Or in config.env file
DEVICE_IDS=6899913485570306485,6906448283393854879
```

**Command Line:**
```bash
./flume-exporter -device-ids="6899913485570306485,6906448283393854879"
```

**Systemd Service:**
```ini
# In /opt/flume-exporter/config.env
DEVICE_IDS=6899913485570306485,6906448283393854879
```

### Benefits

- **Reduced API Calls**: Only query specified devices, reducing API usage
- **Focused Monitoring**: Collect metrics only from devices you care about
- **Performance**: Faster metric collection when you have many devices
- **Cost Control**: Stay within Flume API rate limits more easily
- **Optimized Collection**: Daily total water usage is collected only twice per day (morning and evening) plus on service start, reducing unnecessary API calls

### Finding Your Device IDs

You can find your device IDs in several ways:
1. **Flume App**: Device settings show the device ID
2. **API Response**: The exporter logs show device IDs during startup
3. **Metrics**: Check the `device_id` label in your Prometheus metrics

## Daily Total Water Usage Optimization

The `flume_daily_total_water_usage_gallons` metric is optimized to reduce API calls while maintaining data freshness:

### Collection Schedule

- **Service Start**: Always collected when the exporter starts
- **Morning Collection**: Around 6 AM (5-7 AM window) - captures overnight usage
- **Evening Collection**: Around 6 PM (5-7 PM window) - captures daytime usage
- **New Day**: Automatically collected on the first scrape of each new day

### Benefits

- **Reduced API Calls**: From every scrape to only twice per day
- **Better Rate Limit Management**: Stays well within Flume's 120 requests/hour limit
- **Data Freshness**: Still provides daily updates for trending and analysis
- **Efficient Resource Usage**: Reduces unnecessary data collection during low-usage periods

### How It Works

The exporter tracks when daily total water usage was last collected and only makes API calls when:
1. The service starts up
2. It's time for morning collection (around 6 AM)
3. It's time for evening collection (around 6 PM)
4. A new day begins

## Dynamic Scrape Interval Optimization

The exporter automatically calculates the optimal scrape interval based on the number of devices being monitored to stay within Flume's 120 requests/hour limit:

### Automatic Interval Calculation

| Devices | Optimal Interval | Requests/Hour | Status |
|---------|------------------|---------------|---------|
| 1 device | 2 minutes | 60-90 | ✅ Well under limit |
| 2 devices | 2 minutes | 90-120 | ✅ Under limit |
| 3 devices | 3 minutes | 80-100 | ✅ Under limit |
| 4 devices | 4 minutes | 75-90 | ✅ Well under limit |
| 5+ devices | 5+ minutes | < 72 | ✅ Well under limit |

### How It Works

1. **Device Count Detection**: On startup, the exporter counts how many devices will be processed
2. **Interval Calculation**: Uses formula: `30 × (1 + device_count)` seconds
3. **Smart Bounds**: Ensures interval stays between 1-10 minutes
4. **User Override**: Custom intervals specified via `SCRAPE_INTERVAL` take precedence

### Benefits

- **Automatic Rate Limit Compliance**: No manual calculation needed
- **Optimal Data Freshness**: Fastest possible collection while staying under limits
- **Scalable**: Automatically adjusts as you add/remove devices
- **User Friendly**: Works out-of-the-box with optimal settings

## Authentication Optimization

The exporter optimizes authentication to minimize unnecessary API calls to the `/me` endpoint:

### Smart Token Management

- **Token Expiry Tracking**: Monitors token expiration without making API calls
- **Proactive Refresh**: Refreshes tokens before they expire (within 1 hour)
- **Conditional Validation**: Only validates tokens via API when necessary
- **Persistent Storage**: Saves tokens to disk to avoid re-authentication

### Health Check Endpoints

- **`/health`**: Basic health status without API calls (fast, efficient)
- **`/health/detailed`**: Full health status with API validation (when needed)

### Benefits

- **Reduced API Calls**: Eliminates unnecessary `/me` endpoint calls
- **Faster Health Checks**: Basic health status returns immediately
- **Better Rate Limit Management**: Stays within Flume's API limits
- **Improved Performance**: Faster startup and health monitoring

### How It Works

1. **Token Loading**: Loads existing tokens from disk on startup
2. **Expiry Check**: Uses local time comparison instead of API calls
3. **Smart Refresh**: Refreshes tokens proactively before expiration
4. **Conditional Validation**: Only makes `/me` calls when tokens are expired
5. **Persistent Storage**: Saves tokens to avoid re-authentication on restarts

This optimization significantly reduces the number of API calls, especially for users with frequent health checks or monitoring systems.

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

# Optional: Filter specific devices (comma-separated)
# If not specified, data from all devices will be collected
DEVICE_IDS=6899913485570306485,6906448283393854879
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

# Optional: Filter specific devices (comma-separated)
# If not specified, data from all devices will be collected
export DEVICE_IDS="6899913485570306485,6906448283393854879"
```

## Metrics

### Water Usage Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `flume_current_flow_rate_gallons_per_minute` | Gauge | Current water flow rate (direct from API) | `device_id`, `device_name`, `location` |
| `flume_daily_total_water_usage_gallons` | Gauge | Daily total water usage for each day over time period (collected twice per day) | `device_id`, `device_name`, `location`, `date` |
| `flume_total_water_usage_gallons` | Gauge | Total usage for time period | `device_id`, `device_name`, `location`, `bucket` |

### Device Information Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `flume_device_info` | Gauge | Device information (always 1) | `device_id`, `device_name`, `location`, `device_type` |

### Exporter Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `flume_exporter_scrape_duration_seconds` | Gauge | Time spent scraping API | `endpoint` |
| `flume_exporter_scrape_success` | Gauge | Whether last scrape succeeded (1/0) | `endpoint` |
| `flume_exporter_last_scrape_timestamp_seconds` | Gauge | Unix timestamp of last scrape | `endpoint` |
| `flume_exporter_rate_limit_errors_total` | Counter | Total number of rate limit errors (429) encountered | `endpoint` |

## Example Queries

### Grafana Dashboard Queries

**Current Flow Rate:**
```promql
flume_current_flow_rate_gallons_per_minute
```

**Daily Total Water Usage (30-day history):**
```promql
flume_daily_total_water_usage_gallons
```

**Water Usage Rate of Change:**
```promql
rate(flume_total_water_usage_gallons[5m]) * 60
```

**Rate Limit Errors (429):**
```promql
flume_exporter_rate_limit_errors_total
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

- **Dynamic Optimization**: Automatically calculates optimal scrape intervals based on device count
- **Default Configuration**: Limits API requests to a minimum of 30 seconds apart (120 requests/hour)
- **Configurable**: You can adjust the rate limiting via the `API_MIN_INTERVAL` environment variable or `-api-min-interval` flag
- **Per-Request Limiting**: Each API call (devices, flow rate, water usage) is individually rate-limited
- **Automatic Throttling**: The exporter will automatically wait between requests to stay within limits
- **Rate Limit Monitoring**: Tracks 429 errors to help identify when limits are exceeded

**Example Rate Limiting Configuration:**
```bash
# Conservative: 60 seconds between requests (60 requests/hour)
export API_MIN_INTERVAL=60s

# Default: 30 seconds between requests (120 requests/hour)  
export API_MIN_INTERVAL=30s

# Aggressive: 20 seconds between requests (180 requests/hour - may exceed limits)
export API_MIN_INTERVAL=20s
```

**Note**: With the dynamic interval optimization, the exporter automatically adjusts the scrape interval based on your device count to stay within the 120 requests/hour limit while providing the fastest possible data collection.

## Rate Limit Monitoring

The exporter now includes built-in monitoring for API rate limit violations:

### New Metric: `flume_exporter_rate_limit_errors_total`

This **counter** metric tracks the total number of 429 (Too Many Requests) errors encountered from the Flume API.

**Labels:**
- `endpoint`: Which API endpoint hit the rate limit (e.g., `devices`, `flow_rate`, `daily_total_water_usage`, `water_usage`)

### Monitoring Rate Limit Errors

**Grafana Alert Example:**
```promql
# Alert if rate limit errors occur
flume_exporter_rate_limit_errors_total > 0
```

**Rate of Rate Limit Errors:**
```promql
# How many rate limit errors per hour
rate(flume_exporter_rate_limit_errors_total[1h])
```

**Rate Limit Errors by Endpoint:**
```promql
# Group by endpoint to see which calls are hitting limits
flume_exporter_rate_limit_errors_total
```

### What This Tells You

- **`> 0`**: You're hitting Flume's API rate limits
- **High values**: Your configuration is too aggressive
- **Specific endpoints**: Which API calls are causing issues
- **Trends**: Whether rate limiting is getting worse over time

### Recommended Actions

1. **Increase `API_MIN_INTERVAL`**: Add more delay between API calls
2. **Increase `SCRAPE_INTERVAL`**: Collect data less frequently
3. **Filter devices**: Use `DEVICE_IDS` to monitor fewer devices
4. **Check logs**: Look for "Rate limit exceeded" messages

This monitoring helps you optimize your configuration to stay within Flume's API limits while maintaining the best possible data collection frequency.

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