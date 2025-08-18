package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	// API rate limiting
	APIMinInterval time.Duration

	// Device filtering
	DeviceIDs string
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		ListenAddress:  ":9193",
		MetricsPath:    "/metrics",
		ScrapeInterval: 30 * time.Second,
		Timeout:        10 * time.Second,
		BaseURL:        "https://api.flumewater.com",
		APIMinInterval: 30 * time.Second, // Default: minimum 30 seconds between API requests (120 requests/hour limit)
	}
}

// LoadConfig loads configuration from environment variables and command line flags
func LoadConfig() (*Config, error) {
	config := NewConfig()

	// Define command line flags
	flag.StringVar(&config.ClientID, "client-id", "", "Flume API client ID")
	flag.StringVar(&config.ClientSecret, "client-secret", "", "Flume API client secret")
	flag.StringVar(&config.Username, "username", "", "Flume account email address")
	flag.StringVar(&config.Password, "password", "", "Flume account password")
	flag.StringVar(&config.ListenAddress, "listen-address", config.ListenAddress, "Address to listen on")
	flag.StringVar(&config.MetricsPath, "metrics-path", config.MetricsPath, "Path under which to expose metrics")
	flag.DurationVar(&config.ScrapeInterval, "scrape-interval", config.ScrapeInterval, "Interval between metric scrapes")
	flag.DurationVar(&config.Timeout, "timeout", config.Timeout, "Request timeout")
	flag.StringVar(&config.BaseURL, "base-url", config.BaseURL, "Flume API base URL")
	flag.DurationVar(&config.APIMinInterval, "api-min-interval", config.APIMinInterval, "Minimum interval between Flume API requests")
	flag.StringVar(&config.DeviceIDs, "device-ids", "", "Comma-separated list of device IDs to scrape (e.g., 123,456,789)")

	// Add flag to clear tokens
	clearTokens := flag.Bool("clear-tokens", false, "Clear stored authentication tokens")

	flag.Parse()

	// Handle clear-tokens flag
	if *clearTokens {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Printf("Warning: Could not determine home directory: %v", err)
			homeDir = "."
		}
		tokenFile := filepath.Join(homeDir, ".flume_exporter_tokens.json")

		if err := os.Remove(tokenFile); err != nil {
			if os.IsNotExist(err) {
				log.Println("No token file found to clear")
			} else {
				log.Printf("Warning: Failed to remove token file: %v", err)
			}
		} else {
			log.Println("Authentication tokens cleared successfully")
		}
	}

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
	if val := os.Getenv("SCRAPE_INTERVAL"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.ScrapeInterval = parsed
		} else {
			log.Printf("Warning: Invalid SCRAPE_INTERVAL value '%s', using default: %v", val, config.ScrapeInterval)
		}
	}
	if val := os.Getenv("TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.Timeout = parsed
		} else {
			log.Printf("Warning: Invalid TIMEOUT value '%s', using default: %v", val, config.Timeout)
		}
	}
	if val := os.Getenv("API_MIN_INTERVAL"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.APIMinInterval = parsed
		} else {
			log.Printf("Warning: Invalid API_MIN_INTERVAL value '%s', using default: %v", val, config.APIMinInterval)
		}
	}
	if val := os.Getenv("DEVICE_IDS"); val != "" {
		config.DeviceIDs = val
	}

	// Validate required configuration with helpful error messages
	if config.ClientID == "" {
		return nil, fmt.Errorf("client ID is required (set via --client-id flag or FLUME_CLIENT_ID env var)\n" +
			"Get your API credentials from: https://portal.flumewater.com/ -> Settings -> Generate API Client")
	}
	if config.ClientSecret == "" {
		return nil, fmt.Errorf("client secret is required (set via --client-secret flag or FLUME_CLIENT_SECRET env var)\n" +
			"Get your API credentials from: https://portal.flumewater.com/ -> Settings -> Generate API Client")
	}
	if config.Username == "" {
		return nil, fmt.Errorf("email address is required (set via --username flag or FLUME_USERNAME env var)\n" +
			"This should be the email address you use to log into your Flume account")
	}
	if config.Password == "" {
		return nil, fmt.Errorf("password is required (set via --password flag or FLUME_PASSWORD env var)\n" +
			"This should be the password for your Flume account")
	}

	return config, nil
}

// calculateOptimalScrapeInterval determines the optimal scrape interval based on device count
// to stay under Flume's 120 requests/hour limit
func (c *Config) calculateOptimalScrapeInterval(deviceCount int) time.Duration {
	// Base requests per scrape: 1 (get devices) + deviceCount (flow rate) + deviceCount (daily total when scheduled)
	// Daily total is collected ~2x per day, so average per scrape is minimal
	baseRequestsPerScrape := 1 + deviceCount

	// Target: stay under 120 requests/hour
	// Formula: interval = 3600 seconds / (120 / baseRequestsPerScrape)
	// Simplified: interval = 30 * baseRequestsPerScrape seconds
	optimalIntervalSeconds := 30 * baseRequestsPerScrape

	// Convert to time.Duration
	optimalInterval := time.Duration(optimalIntervalSeconds) * time.Second

	// Ensure minimum interval of 2 minutes and maximum of 10 minutes
	// 2 minutes provides safer margin below rate limit for all device counts
	if optimalInterval < 2*time.Minute {
		optimalInterval = 2 * time.Minute
	} else if optimalInterval > 10*time.Minute {
		optimalInterval = 10 * time.Minute
	}

	return optimalInterval
}

// GetScrapeInterval returns the optimal scrape interval based on device count
func (c *Config) GetScrapeInterval(deviceCount int) time.Duration {
	// If user specified a custom interval, use that
	if c.ScrapeInterval != 30*time.Second {
		return c.ScrapeInterval
	}

	// Otherwise, calculate optimal interval
	return c.calculateOptimalScrapeInterval(deviceCount)
}
