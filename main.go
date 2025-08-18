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

		// Get authentication status
		authStatus := client.GetAuthenticationStatus()

		// Try to validate authentication
		authValid := true
		if err := client.ValidateAuthentication(); err != nil {
			authValid = false
			authStatus["validation_error"] = err.Error()
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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html>
<head><title>Flume Water Prometheus Exporter</title></head>
<body>
<h1>Flume Water Prometheus Exporter</h1>
<p><a href="` + config.MetricsPath + `">Metrics</a></p>
<p>This exporter collects water usage metrics from the Flume API and exposes them as Prometheus metrics.</p>
<h2>Available Metrics:</h2>
<ul>
<li><code>flume_current_flow_rate_gallons_per_minute</code> - Current water flow rate</li>
<li><code>flume_hourly_water_usage_gallons</code> - Hourly water usage</li>
<li><code>flume_daily_water_usage_gallons</code> - Daily water usage</li>
<li><code>flume_daily_total_water_usage_gallons</code> - Daily total water usage for each day over time period</li>
<li><code>flume_total_water_usage_gallons</code> - Total water usage for time periods</li>
<li><code>flume_device_info</code> - Device information</li>
<li><code>flume_exporter_scrape_duration_seconds</code> - Time spent scraping API</li>
<li><code>flume_exporter_scrape_success</code> - Scrape success status</li>
<li><code>flume_exporter_last_scrape_timestamp_seconds</code> - Last scrape timestamp</li>
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

		// Validate authentication before starting
		log.Println("Validating authentication...")
		if err := client.ValidateAuthentication(); err != nil {
			log.Printf("Authentication validation failed: %v", err)
			log.Println("Attempting to authenticate...")

			// Try to authenticate with retry
			if err := client.AuthenticateWithRetry(3); err != nil {
				log.Printf("Failed to authenticate after retries: %v", err)
				log.Println("Metrics endpoint is still available, but data collection will fail")
				return
			}

			log.Println("Authentication successful!")
		} else {
			log.Println("Authentication validation successful!")
		}

		// Start periodic metric collection only after successful authentication
		log.Println("Starting periodic metric collection...")
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
