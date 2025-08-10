#!/bin/bash

# Flume Water Prometheus Exporter - Raspberry Pi Installation Script
# This script installs the flume-exporter as a systemd service on Raspberry Pi

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   print_error "This script should not be run as root. Please run as a regular user with sudo access."
   exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21+ first."
    print_status "You can install Go with: sudo apt update && sudo apt install golang-go"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
GO_MAJOR=$(echo $GO_VERSION | cut -d. -f1)
GO_MINOR=$(echo $GO_VERSION | cut -d. -f2)

if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 21 ]); then
    print_error "Go version $GO_VERSION is too old. Please install Go 1.21+ first."
    exit 1
fi

print_status "Go version $GO_VERSION detected - OK"

# Get current directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check if we're in the right directory
if [ ! -f "main.go" ] || [ ! -f "flume-exporter.service" ]; then
    print_error "This script must be run from the flume-water-prometheus-exporter directory"
    exit 1
fi

print_status "Building flume-exporter for ARM64..."
GOOS=linux GOARCH=arm64 go build -o flume-exporter

if [ ! -f "flume-exporter" ]; then
    print_error "Build failed - flume-exporter binary not found"
    exit 1
fi

print_status "Build successful"

# Create configuration directory
print_status "Creating configuration directory..."
sudo mkdir -p /etc/flume-exporter

# Check if config file already exists
if [ -f "/etc/flume-exporter/config.env" ]; then
    print_warning "Configuration file already exists at /etc/flume-exporter/config.env"
    read -p "Do you want to overwrite it? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_status "Keeping existing configuration file"
    else
        print_status "Overwriting existing configuration file"
        sudo rm /etc/flume-exporter/config.env
    fi
fi

# Create configuration file if it doesn't exist
if [ ! -f "/etc/flume-exporter/config.env" ]; then
    print_status "Creating configuration file..."
    sudo tee /etc/flume-exporter/config.env > /dev/null <<EOF
# Flume API Credentials
FLUME_CLIENT_ID=your_client_id_here
FLUME_CLIENT_SECRET=your_client_secret_here
FLUME_USERNAME=your_username_here
FLUME_PASSWORD=your_password_here

# Server Configuration
LISTEN_ADDRESS=:9193
SCRAPE_INTERVAL=30s
API_MIN_INTERVAL=30s
EOF

    print_warning "Please edit /etc/flume-exporter/config.env with your actual Flume credentials"
    print_status "You can edit it with: sudo nano /etc/flume-exporter/config.env"
fi

# Set proper permissions
print_status "Setting configuration file permissions..."
sudo chown root:root /etc/flume-exporter/config.env
sudo chmod 600 /etc/flume-exporter/config.env

# Install binary
print_status "Installing flume-exporter binary..."
sudo cp flume-exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/flume-exporter

# Install systemd service
print_status "Installing systemd service..."
sudo cp flume-exporter.service /etc/systemd/system/
sudo systemctl daemon-reload

# Enable and start service
print_status "Enabling and starting flume-exporter service..."
sudo systemctl enable flume-exporter
sudo systemctl start flume-exporter

# Wait a moment for service to start
sleep 3

# Check service status
if sudo systemctl is-active --quiet flume-exporter; then
    print_status "flume-exporter service is running successfully!"
    print_status "Service status:"
    sudo systemctl status flume-exporter --no-pager -l
else
    print_error "flume-exporter service failed to start"
    print_status "Checking service logs:"
    sudo journalctl -u flume-exporter --no-pager -l
    exit 1
fi

print_status "Installation completed successfully!"
print_status ""
print_status "Next steps:"
print_status "1. Edit configuration: sudo nano /etc/flume-exporter/config.env"
print_status "2. Restart service: sudo systemctl restart flume-exporter"
print_status "3. Test metrics: curl http://localhost:9193/metrics"
print_status "4. View logs: sudo journalctl -u flume-exporter -f"
print_status ""
print_status "The service will automatically start on boot"
print_status "To stop the service: sudo systemctl stop flume-exporter"
print_status "To disable auto-start: sudo systemctl disable flume-exporter"