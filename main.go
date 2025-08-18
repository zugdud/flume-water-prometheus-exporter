package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	log.Println("Starting Flume Water Prometheus Exporter...")

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded:")
	log.Printf("  Listen Address: %s", config.ListenAddress)
	log.Printf("  Metrics Path: %s", config.MetricsPath)
	log.Printf("  Scrape Interval: %s", config.ScrapeInterval)
	log.Printf("  Timeout: %s", config.Timeout)
	log.Printf("  Base URL: %s", config.BaseURL)
	log.Printf("  API Min Interval: %s", config.APIMinInterval)
	if config.DeviceIDs != "" {
		log.Printf("  Device IDs Filter: %s", config.DeviceIDs)
	} else {
		log.Printf("  Device IDs Filter: All devices")
	}

	// Create Flume client
	client := NewFlumeClient(config)

	// Create exporter
	exporter := NewFlumeExporter(client, config)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle(config.MetricsPath, promhttp.Handler())

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get authentication status without making API calls
		authStatus := client.GetAuthenticationStatus()

		// Only validate authentication if we need to
		authValid := true

		if client.needsAuthentication() {
			log.Printf("Health check: Authentication needed, validating...")
			if err := client.ValidateAuthentication(); err != nil {
				authValid = false
				authStatus["validation_error"] = err.Error()
			}
		} else {
			log.Printf("Health check: Token appears valid, skipping API validation")
			authStatus["validation_skipped"] = "token_valid"
		}

		healthData := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"authentication": map[string]interface{}{
				"valid":  authValid,
				"status": authStatus,
			},
			"config": map[string]interface{}{
				"base_url":         config.BaseURL,
				"username":         config.Username,
				"client_id":        config.ClientID,
				"scrape_interval":  config.ScrapeInterval.String(),
				"device_filtering": config.DeviceIDs != "",
				"device_ids":       config.DeviceIDs,
			},
		}

		if !authValid {
			healthData["status"] = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		jsonData, _ := json.MarshalIndent(healthData, "", "  ")
		w.Write(jsonData)
	})

	// Add detailed health check endpoint that includes API validation
	mux.HandleFunc("/health/detailed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get detailed authentication status including API validation
		authStatus := client.GetDetailedAuthenticationStatus()

		authValid := authStatus["api_validation"] == "success" || authStatus["api_validation"] == "skipped"

		healthData := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"authentication": map[string]interface{}{
				"valid":  authValid,
				"status": authStatus,
			},
			"config": map[string]interface{}{
				"base_url":         config.BaseURL,
				"username":         config.Username,
				"client_id":        config.ClientID,
				"scrape_interval":  config.ScrapeInterval.String(),
				"device_filtering": config.DeviceIDs != "",
				"device_ids":       config.DeviceIDs,
			},
		}

		if !authValid {
			healthData["status"] = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		jsonData, _ := json.MarshalIndent(healthData, "", "  ")
		w.Write(jsonData)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html>
<head><title>Flume Water Prometheus Exporter</title></head>
<body>
<h1>Flume Water Prometheus Exporter</h1>
<p><a href="` + config.MetricsPath + `">Metrics</a></p>
<p>This exporter collects water usage metrics from the Flume API and exposes them as Prometheus metrics.</p>
<h2>Available Endpoints:</h2>
<ul>
<li><a href="` + config.MetricsPath + `">Metrics</a> - Prometheus metrics</li>
<li><a href="/health">Health Check</a> - Basic health status (no API calls)</li>
<li><a href="/health/detailed">Detailed Health</a> - Full health status with API validation</li>
</ul>
</body>
</html>`))
	})

	server := &http.Server{
		Addr:    config.ListenAddress,
		Handler: mux,
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("Starting HTTP server on %s", config.ListenAddress)
		log.Printf("Metrics available at http://%s%s", config.ListenAddress, config.MetricsPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start authentication in background
	go func() {
		log.Println("Starting authentication in background...")

		// Check if we need authentication before starting
		if client.needsAuthentication() {
			log.Println("Authentication needed, starting...")

			// Try to authenticate with retry
			if err := client.AuthenticateWithRetry(3); err != nil {
				log.Printf("Failed to authenticate after retries: %v", err)
				log.Println("Metrics endpoint is still available, but data collection will fail")
				return
			}

			log.Println("Authentication successful!")
		} else {
			log.Println("Valid tokens found, authentication not needed")
		}

		// Get initial device count to calculate optimal interval
		devices, err := client.GetDevices()
		if err != nil {
			log.Printf("Failed to get initial device count: %v", err)
			log.Println("Using default scrape interval")
		} else {
			// Count devices that will be processed
			deviceCount := len(devices)
			if config.DeviceIDs != "" {
				deviceCount = 0
				for _, device := range devices {
					if exporter.shouldProcessDevice(device.ID) {
						deviceCount++
					}
				}
			}

			// Calculate optimal interval
			optimalInterval := config.GetScrapeInterval(deviceCount)
			log.Printf("Device count: %d, Optimal scrape interval: %s", deviceCount, optimalInterval)

			// Update config with optimal interval
			config.ScrapeInterval = optimalInterval
		}

		// Start periodic metric collection
		log.Println("Starting periodic metric collection...")
		log.Printf("Using scrape interval: %s", config.ScrapeInterval)
		exporter.StartPeriodicCollection(config.ScrapeInterval)
	}()

	// Wait for shutdown signal
	<-shutdown
	log.Println("Shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Exporter stopped")
}

// RateLimiter ensures that operations are not performed more frequently than a specified interval
type RateLimiter struct {
	interval time.Duration
	last     time.Time
	mutex    sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the specified minimum interval
func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{
		interval: interval,
		last:     time.Time{}, // Zero time means no previous operation
	}
}

// Wait blocks until enough time has passed since the last operation
func (rl *RateLimiter) Wait() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	if !rl.last.IsZero() {
		// Calculate how long to wait
		elapsed := now.Sub(rl.last)
		if elapsed < rl.interval {
			waitTime := rl.interval - elapsed
			time.Sleep(waitTime)
			now = time.Now() // Update now after sleeping
		}
	}

	rl.last = now
}

// GetInterval returns the configured interval
func (rl *RateLimiter) GetInterval() time.Duration {
	return rl.interval
}
