package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	// Create Flume client
	client := NewFlumeClient(config)

	// Create exporter
	exporter := NewFlumeExporter(client)

	// Start periodic metric collection
	exporter.StartPeriodicCollection(config.ScrapeInterval)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle(config.MetricsPath, promhttp.Handler())
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