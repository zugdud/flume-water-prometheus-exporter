#!/bin/bash
set -e

# Flume Water Prometheus Exporter - Raspberry Pi Installation Script
# This script installs and configures the Flume exporter as a systemd service

echo "🌊 Flume Water Prometheus Exporter - Raspberry Pi Installer"
echo "==========================================================="

# Detect architecture
ARCH=$(uname -m)
PI_MODEL=$(tr -d '\0' < /proc/device-tree/model 2>/dev/null || echo "Unknown")

case $ARCH in
    aarch64|arm64)
        BINARY="flume-exporter-linux-arm64"
        echo "✓ Detected 64-bit ARM architecture ($PI_MODEL)"
        if [[ "$PI_MODEL" == *"Pi 5"* ]]; then
            echo "✓ Raspberry Pi 5 detected - using optimized ARM64 build"
        fi
        ;;
    armv7l|armhf)
        BINARY="flume-exporter-linux-arm32"
        echo "✓ Detected 32-bit ARM architecture ($PI_MODEL)"
        ;;
    *)
        echo "❌ Unsupported architecture: $ARCH"
        echo "This script is designed for Raspberry Pi (ARM) devices only."
        echo "Detected model: $PI_MODEL"
        exit 1
        ;;
esac

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   echo "❌ This script should not be run as root"
   echo "Please run as a regular user (e.g., pi) with sudo access"
   exit 1
fi

# Check for required commands
command -v systemctl >/dev/null 2>&1 || { echo "❌ systemctl is required but not installed. Aborting." >&2; exit 1; }

echo
echo "Step 1: Installing binary..."

# Check for available binaries (prefer Pi 5 optimized)
PI5_BINARY="flume-exporter-pi5-arm64"
GENERIC_BINARY="flume-exporter"

if [[ "$PI_MODEL" == *"Pi 5"* && -f "./$PI5_BINARY" ]]; then
    echo "✓ Found Pi 5 optimized binary: $PI5_BINARY"
    sudo cp "./$PI5_BINARY" /usr/local/bin/flume-exporter
elif [[ -f "./$GENERIC_BINARY" ]]; then
    echo "✓ Found generic binary: $GENERIC_BINARY"
    sudo cp "./$GENERIC_BINARY" /usr/local/bin/flume-exporter
elif [[ -f "./$BINARY" ]]; then
    echo "✓ Found $BINARY in current directory"
    sudo cp "./$BINARY" /usr/local/bin/flume-exporter
else
    echo "❌ No suitable binary found in current directory"
    echo "Available options to build:"
    echo "  ./build-pi5.sh                                                         # Pi 5 optimized"
    echo "  GOOS=linux GOARCH=arm64 go build -o flume-exporter-linux-arm64 .      # 64-bit ARM"
    echo "  GOOS=linux GOARCH=arm GOARM=7 go build -o flume-exporter-linux-arm32 . # 32-bit ARM"
    exit 1
fi

# Make executable
sudo chmod +x /usr/local/bin/flume-exporter
echo "✓ Binary installed to /usr/local/bin/flume-exporter"

echo
echo "Step 2: Creating configuration directory..."
sudo mkdir -p /etc/flume-exporter
echo "✓ Created /etc/flume-exporter"

# Create config file if it doesn't exist
if [[ ! -f /etc/flume-exporter/config.env ]]; then
    echo
    echo "Step 3: Creating configuration file..."
    
    # Prompt for credentials
    echo "Please enter your Flume API credentials:"
    read -p "Client ID: " CLIENT_ID
    read -p "Client Secret: " CLIENT_SECRET
    read -p "Username: " USERNAME
    read -s -p "Password: " PASSWORD
    echo
    
    # Create config file
    sudo tee /etc/flume-exporter/config.env > /dev/null <<EOF
# Flume API Credentials
FLUME_CLIENT_ID=$CLIENT_ID
FLUME_CLIENT_SECRET=$CLIENT_SECRET
FLUME_USERNAME=$USERNAME
FLUME_PASSWORD=$PASSWORD

# Server Configuration
LISTEN_ADDRESS=:8080
METRICS_PATH=/metrics
BASE_URL=https://api.flumewater.com
EOF
    
    # Secure the config file
    sudo chown root:root /etc/flume-exporter/config.env
    sudo chmod 600 /etc/flume-exporter/config.env
    echo "✓ Configuration saved to /etc/flume-exporter/config.env"
else
    echo "✓ Configuration file already exists at /etc/flume-exporter/config.env"
fi

echo
echo "Step 4: Installing systemd service..."

# Install service file
if [[ -f "./flume-exporter.service" ]]; then
    sudo cp ./flume-exporter.service /etc/systemd/system/
else
    # Create service file if not present
    sudo tee /etc/systemd/system/flume-exporter.service > /dev/null <<'EOF'
[Unit]
Description=Flume Water Prometheus Exporter
Documentation=https://github.com/flume-water-prometheus-exporter
After=network.target
Wants=network.target

[Service]
Type=simple
User=pi
Group=pi
ExecStart=/usr/local/bin/flume-exporter
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=flume-exporter

# Use environment file for configuration
EnvironmentFile=/etc/flume-exporter/config.env

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/tmp

[Install]
WantedBy=multi-user.target
EOF
fi

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable flume-exporter
echo "✓ Service installed and enabled"

echo
echo "Step 5: Starting service..."
sudo systemctl start flume-exporter

# Wait a moment and check status
sleep 3
if sudo systemctl is-active --quiet flume-exporter; then
    echo "✓ Service started successfully"
else
    echo "❌ Service failed to start. Check logs with:"
    echo "  sudo journalctl -u flume-exporter -f"
    exit 1
fi

echo
echo "🎉 Installation completed successfully!"
echo
echo "Service Status:"
sudo systemctl status flume-exporter --no-pager -l

echo
echo "📊 Access your metrics at: http://$(hostname -I | awk '{print $1}'):8080/metrics"
echo
echo "Useful commands:"
echo "  sudo systemctl status flume-exporter    # Check service status"
echo "  sudo systemctl stop flume-exporter      # Stop service"
echo "  sudo systemctl start flume-exporter     # Start service"
echo "  sudo systemctl restart flume-exporter   # Restart service"
echo "  sudo journalctl -u flume-exporter -f    # View logs"
echo "  sudo systemctl disable flume-exporter   # Disable auto-start"
echo
echo "Configuration file: /etc/flume-exporter/config.env"
echo "Service file: /etc/systemd/system/flume-exporter.service"
echo
echo "🌊 Happy water monitoring! 🌊"