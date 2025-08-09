#!/bin/bash
set -e

# Build script for Raspberry Pi 5 with Go 1.23+
# This script can be run directly on a Pi 5 or used for cross-compilation

echo "🌊 Building Flume Water Prometheus Exporter for Raspberry Pi 5"
echo "============================================================="

# Check Go version
GO_VERSION=$(go version 2>/dev/null || echo "not found")
echo "Go version: $GO_VERSION"

if [[ "$GO_VERSION" == "not found" ]]; then
    echo "❌ Go is not installed. Please install Go 1.23+ first."
    echo "See README.md for installation instructions."
    exit 1
fi

# Extract version number
GO_VER=$(echo $GO_VERSION | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
MAJOR=$(echo $GO_VER | cut -d. -f1)
MINOR=$(echo $GO_VER | cut -d. -f2)

if [[ $MAJOR -lt 1 ]] || [[ $MAJOR -eq 1 && $MINOR -lt 23 ]]; then
    echo "⚠️  Warning: Go $GO_VER detected. Go 1.23+ is recommended for optimal performance."
fi

# Detect if we're running on Pi 5
PI_MODEL=$(tr -d '\0' < /proc/device-tree/model 2>/dev/null || echo "Unknown")
if [[ "$PI_MODEL" == *"Pi 5"* ]]; then
    echo "✓ Running on Raspberry Pi 5"
    BUILD_MODE="native"
else
    echo "ℹ️  Cross-compiling for Raspberry Pi 5"
    BUILD_MODE="cross"
    export GOOS=linux
    export GOARCH=arm64
fi

echo "Build mode: $BUILD_MODE"
echo

# Clean previous builds
echo "Cleaning previous builds..."
rm -f flume-exporter flume-exporter-*

# Update dependencies
echo "Updating dependencies..."
go mod tidy

# Build optimized for Pi 5
echo "Building flume-exporter..."
if [[ "$BUILD_MODE" == "native" ]]; then
    # Native build with optimizations
    go build -ldflags="-s -w" -o flume-exporter .
    echo "✓ Native build completed: flume-exporter"
else
    # Cross-compile build
    go build -ldflags="-s -w" -o flume-exporter-pi5-arm64 .
    echo "✓ Cross-compilation completed: flume-exporter-pi5-arm64"
fi

# Show binary info
echo
echo "Build summary:"
if [[ "$BUILD_MODE" == "native" ]]; then
    ls -lh flume-exporter
    file flume-exporter
else
    ls -lh flume-exporter-pi5-arm64
    file flume-exporter-pi5-arm64
fi

echo
echo "🎉 Build completed successfully!"

if [[ "$BUILD_MODE" == "native" ]]; then
    echo
    echo "Next steps:"
    echo "1. Test the binary: ./flume-exporter --help"
    echo "2. Copy configuration: cp config.example .env"
    echo "3. Edit .env with your Flume credentials"
    echo "4. Run: ./flume-exporter"
    echo "5. Install as service: sudo ./install-raspberry-pi.sh"
else
    echo
    echo "Next steps:"
    echo "1. Transfer to Pi 5: scp flume-exporter-pi5-arm64 pi@your-pi:/home/pi/"
    echo "2. SSH to Pi and run: chmod +x flume-exporter-pi5-arm64"
    echo "3. Test: ./flume-exporter-pi5-arm64 --help"
fi