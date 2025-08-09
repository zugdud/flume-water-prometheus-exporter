# Flume Water Prometheus Exporter Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=flume-exporter

# Version info
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

.PHONY: all build clean test deps help

# Default target
all: clean deps test build-all

# Build for current platform
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

# Build for all supported platforms
build-all: build-linux-arm64 build-linux-arm32 build-linux-amd64 build-windows-amd64 build-darwin-amd64

# Build for Raspberry Pi 64-bit
build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .

# Build for Raspberry Pi 32-bit
build-linux-arm32:
	GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm32 .

# Build for Linux x64
build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .

# Build for Windows x64
build-windows-amd64:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

# Build for macOS x64
build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)*

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOGET) -d -v ./...
	$(GOCMD) mod tidy

# Install for current platform
install: build
	sudo cp $(BINARY_NAME) /usr/local/bin/

# Create release archive
release: clean build-all
	mkdir -p release
	cp $(BINARY_NAME)-linux-arm64 release/
	cp $(BINARY_NAME)-linux-arm32 release/
	cp $(BINARY_NAME)-linux-amd64 release/
	cp $(BINARY_NAME)-windows-amd64.exe release/
	cp $(BINARY_NAME)-darwin-amd64 release/
	cp install-raspberry-pi.sh release/
	cp flume-exporter.service release/
	cp README.md release/
	cp config.example release/
	cd release && tar -czf ../flume-exporter-$(VERSION).tar.gz *

# Show available targets
help:
	@echo "Available targets:"
	@echo "  build              - Build for current platform"
	@echo "  build-all          - Build for all supported platforms"
	@echo "  build-linux-arm64  - Build for Raspberry Pi 64-bit"
	@echo "  build-linux-arm32  - Build for Raspberry Pi 32-bit"
	@echo "  build-linux-amd64  - Build for Linux x64"
	@echo "  build-windows-amd64 - Build for Windows x64"
	@echo "  build-darwin-amd64 - Build for macOS x64"
	@echo "  clean              - Clean build artifacts"
	@echo "  test               - Run tests"
	@echo "  deps               - Download dependencies"
	@echo "  install            - Install for current platform"
	@echo "  release            - Create release archive"
	@echo "  help               - Show this help"