package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Config holds all configuration options for the exporter
type Config struct {
	// Flume API credentials
	ClientID     string
	ClientSecret string
	Username     string
	Password     string

	// Server configuration
	ListenAddress string
	MetricsPath   string

	// Scrape configuration
	ScrapeInterval time.Duration
	Timeout        time.Duration

	// Flume API configuration
	BaseURL string
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		ListenAddress:  ":9193",
		MetricsPath:    "/metrics",
		ScrapeInterval: 30 * time.Second,
		Timeout:        10 * time.Second,
		BaseURL:        "https://api.flumewater.com",
	}
}

// LoadConfig loads configuration from environment variables and command line flags
func LoadConfig() (*Config, error) {
	config := NewConfig()

	// Define command line flags
	flag.StringVar(&config.ClientID, "client-id", "", "Flume API client ID")
	flag.StringVar(&config.ClientSecret, "client-secret", "", "Flume API client secret")
	flag.StringVar(&config.Username, "username", "", "Flume username")
	flag.StringVar(&config.Password, "password", "", "Flume password")
	flag.StringVar(&config.ListenAddress, "listen-address", config.ListenAddress, "Address to listen on")
	flag.StringVar(&config.MetricsPath, "metrics-path", config.MetricsPath, "Path under which to expose metrics")
	flag.DurationVar(&config.ScrapeInterval, "scrape-interval", config.ScrapeInterval, "Interval between metric scrapes")
	flag.DurationVar(&config.Timeout, "timeout", config.Timeout, "Request timeout")
	flag.StringVar(&config.BaseURL, "base-url", config.BaseURL, "Flume API base URL")

	flag.Parse()

	// Override with environment variables if present
	if val := os.Getenv("FLUME_CLIENT_ID"); val != "" {
		config.ClientID = val
	}
	if val := os.Getenv("FLUME_CLIENT_SECRET"); val != "" {
		config.ClientSecret = val
	}
	if val := os.Getenv("FLUME_USERNAME"); val != "" {
		config.Username = val
	}
	if val := os.Getenv("FLUME_PASSWORD"); val != "" {
		config.Password = val
	}
	if val := os.Getenv("LISTEN_ADDRESS"); val != "" {
		config.ListenAddress = val
	}
	if val := os.Getenv("METRICS_PATH"); val != "" {
		config.MetricsPath = val
	}
	if val := os.Getenv("BASE_URL"); val != "" {
		config.BaseURL = val
	}

	// Validate required configuration
	if config.ClientID == "" {
		return nil, fmt.Errorf("client ID is required (set via --client-id flag or FLUME_CLIENT_ID env var)")
	}
	if config.ClientSecret == "" {
		return nil, fmt.Errorf("client secret is required (set via --client-secret flag or FLUME_CLIENT_SECRET env var)")
	}
	if config.Username == "" {
		return nil, fmt.Errorf("username is required (set via --username flag or FLUME_USERNAME env var)")
	}
	if config.Password == "" {
		return nil, fmt.Errorf("password is required (set via --password flag or FLUME_PASSWORD env var)")
	}

	return config, nil
}
