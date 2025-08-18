package main

import (
	"log"
	"strings"
	"time"

	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the Flume exporter
type Metrics struct {
	// Current flow rate metrics
	currentFlowRate *prometheus.GaugeVec

	// Water usage metrics
	totalWaterUsage      *prometheus.GaugeVec
	dailyTotalWaterUsage *prometheus.GaugeVec

	// Device info metrics
	deviceInfo *prometheus.GaugeVec

	// Exporter metrics
	scrapeDuration *prometheus.GaugeVec
	scrapeSuccess  *prometheus.GaugeVec
	lastScrapeTime *prometheus.GaugeVec

	// API rate limit metrics
	rateLimitErrors *prometheus.CounterVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		currentFlowRate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_current_flow_rate_gallons_per_minute",
				Help: "Current water flow rate in gallons per minute",
			},
			[]string{"device_id", "device_name", "location"},
		),

		totalWaterUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_total_water_usage_gallons",
				Help: "Total water usage in gallons for a specific time period",
			},
			[]string{"device_id", "device_name", "location", "bucket"},
		),

		dailyTotalWaterUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_daily_total_water_usage_gallons",
				Help: "Total water usage in gallons for each day over a time period",
			},
			[]string{"device_id", "device_name", "location", "date"},
		),

		deviceInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_device_info",
				Help: "Information about Flume devices",
			},
			[]string{"device_id", "device_name", "location", "device_type"},
		),

		scrapeDuration: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_exporter_scrape_duration_seconds",
				Help: "Time spent scraping Flume API",
			},
			[]string{"endpoint"},
		),

		scrapeSuccess: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_exporter_scrape_success",
				Help: "Whether the last scrape was successful",
			},
			[]string{"endpoint"},
		),

		lastScrapeTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_exporter_last_scrape_timestamp_seconds",
				Help: "Unix timestamp of the last scrape",
			},
			[]string{"endpoint"},
		),

		rateLimitErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "flume_exporter_rate_limit_errors_total",
				Help: "Total number of rate limit errors encountered during Flume API scraping",
			},
			[]string{"endpoint"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.currentFlowRate,
		m.totalWaterUsage,
		m.dailyTotalWaterUsage,
		m.deviceInfo,
		m.scrapeDuration,
		m.scrapeSuccess,
		m.lastScrapeTime,
		m.rateLimitErrors,
	)

	// Initialize rate limit error metric to 0 for common endpoints
	// This ensures the metric is visible in Prometheus even before any errors occur
	commonEndpoints := []string{"devices", "flow_rate", "daily_total_water_usage", "water_usage"}
	for _, endpoint := range commonEndpoints {
		m.rateLimitErrors.WithLabelValues(endpoint).Add(0)
	}

	return m
}

// UpdateCurrentFlowRate updates the current flow rate metric
func (m *Metrics) UpdateCurrentFlowRate(deviceID, deviceName, location string, flowRate float64) {
	m.currentFlowRate.WithLabelValues(deviceID, deviceName, location).Set(flowRate)
}

// UpdateWaterUsage updates water usage metrics from query response
func (m *Metrics) UpdateWaterUsage(deviceID, deviceName, location string, queryResp *QueryResponse) {
	for _, data := range queryResp.Data {
		bucket := data.Bucket

		// Calculate total usage for this time period
		var totalUsage float64
		for _, waterUsage := range data.WaterUsage {
			totalUsage += waterUsage.Value
		}

		// Update the appropriate metric based on bucket type
		switch bucket {
		case "HR":
			m.totalWaterUsage.WithLabelValues(deviceID, deviceName, location, bucket).Set(totalUsage)
		case "DAY":
			m.totalWaterUsage.WithLabelValues(deviceID, deviceName, location, bucket).Set(totalUsage)
		}
	}
}

// UpdateDailyTotalWaterUsage updates the daily total water usage metric for a specific date
func (m *Metrics) UpdateDailyTotalWaterUsage(deviceID, deviceName, location, date string, usage float64) {
	m.dailyTotalWaterUsage.WithLabelValues(deviceID, deviceName, location, date).Set(usage)
}

// UpdateDeviceInfo updates device information metric
func (m *Metrics) UpdateDeviceInfo(device Device, deviceName string) {
	deviceType := "unknown"
	switch device.Type {
	case 1:
		deviceType = "bridge"
	case 2:
		deviceType = "sensor"
	}

	m.deviceInfo.WithLabelValues(
		device.ID,
		deviceName,
		device.Location.Name,
		deviceType,
	).Set(1)
}

// RecordScrapeMetrics records metrics about a scrape operation
func (m *Metrics) RecordScrapeMetrics(endpoint string, duration time.Duration, success bool) {
	m.scrapeDuration.WithLabelValues(endpoint).Set(duration.Seconds())
	if success {
		m.scrapeSuccess.WithLabelValues(endpoint).Set(1)
	} else {
		m.scrapeSuccess.WithLabelValues(endpoint).Set(0)
	}
	m.lastScrapeTime.WithLabelValues(endpoint).Set(float64(time.Now().Unix()))
}

// RecordRateLimitError records when a rate limit error (429) is encountered
func (m *Metrics) RecordRateLimitError(endpoint string) {
	m.rateLimitErrors.WithLabelValues(endpoint).Inc()
}

// FlumeExporter handles the collection of metrics from Flume API
type FlumeExporter struct {
	client  *FlumeClient
	metrics *Metrics
	config  *Config

	// Track when daily total water usage was last collected
	lastDailyTotalCollection time.Time
	dailyCollectionMutex     sync.Mutex
}

// NewFlumeExporter creates a new Flume exporter
func NewFlumeExporter(client *FlumeClient, config *Config, metrics *Metrics) *FlumeExporter {
	return &FlumeExporter{
		client:  client,
		metrics: metrics,
		config:  config,
	}
}

// shouldProcessDevice checks if a device should be processed based on DeviceIDs configuration
func (e *FlumeExporter) shouldProcessDevice(deviceID string) bool {
	// If no DeviceIDs specified, process all devices
	if e.config.DeviceIDs == "" {
		return true
	}

	// Parse comma-separated device IDs
	deviceIDs := strings.Split(e.config.DeviceIDs, ",")
	for _, id := range deviceIDs {
		if strings.TrimSpace(id) == deviceID {
			return true
		}
	}
	return false
}

// shouldCollectDailyTotalWaterUsage checks if daily total water usage should be collected
// Collects twice per day: once in the morning (around 6 AM) and once in the evening (around 6 PM)
func (e *FlumeExporter) shouldCollectDailyTotalWaterUsage() bool {
	e.dailyCollectionMutex.Lock()
	defer e.dailyCollectionMutex.Unlock()

	now := time.Now()

	// If this is the first collection (zero time), always collect
	if e.lastDailyTotalCollection.IsZero() {
		e.lastDailyTotalCollection = now
		return true
	}

	// Check if we've already collected today
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	lastCollectionDay := time.Date(e.lastDailyTotalCollection.Year(), e.lastDailyTotalCollection.Month(), e.lastDailyTotalCollection.Day(), 0, 0, 0, 0, e.lastDailyTotalCollection.Location())

	// If it's a new day, collect
	if !today.Equal(lastCollectionDay) {
		e.lastDailyTotalCollection = now
		return true
	}

	// If it's the same day, check if we've collected twice already
	// First collection: around 6 AM (5-7 AM window)
	// Second collection: around 6 PM (5-7 PM window)
	hour := now.Hour()

	// Check if we're in the morning window (5-7 AM) and haven't collected yet this morning
	if hour >= 5 && hour <= 7 {
		// Check if we've already collected this morning (before 12 PM)
		if e.lastDailyTotalCollection.Hour() < 12 {
			return false // Already collected this morning
		}
		e.lastDailyTotalCollection = now
		return true
	}

	// Check if we're in the evening window (5-7 PM) and haven't collected yet this evening
	if hour >= 17 && hour <= 19 {
		// Check if we've already collected this evening (after 12 PM)
		if e.lastDailyTotalCollection.Hour() >= 12 {
			return false // Already collected this evening
		}
		e.lastDailyTotalCollection = now
		return true
	}

	return false
}

// CollectMetrics collects all metrics from the Flume API
func (e *FlumeExporter) CollectMetrics() {
	log.Println("Starting metric collection...")

	// Get devices
	start := time.Now()
	devices, err := e.client.GetDevices()
	duration := time.Since(start)

	if err != nil {
		log.Printf("Error getting devices: %v", err)
		e.metrics.RecordScrapeMetrics("devices", duration, false)
		return
	}

	e.metrics.RecordScrapeMetrics("devices", duration, true)
	log.Printf("Found %d devices", len(devices))

	// Count devices that will be processed
	processedCount := 0
	if e.config.DeviceIDs != "" {
		for _, device := range devices {
			if e.shouldProcessDevice(device.ID) {
				processedCount++
			}
		}
		log.Printf("Device filtering active: %d of %d devices will be processed", processedCount, len(devices))
	}

	// Process each device
	for _, device := range devices {
		log.Printf("Processing device %s - Type: %d, Location: '%s'", device.ID, device.Type, device.Location.Name)

		// Check if this device should be processed based on DeviceIDs configuration
		if !e.shouldProcessDevice(device.ID) {
			log.Printf("Skipping device %s (not in DeviceIDs filter)", device.ID)
			continue
		}

		// Update device info
		// Use device ID as device name if location name is empty, otherwise use location name
		deviceName := device.Location.Name
		if deviceName == "" {
			deviceName = device.ID
		}
		e.metrics.UpdateDeviceInfo(device, deviceName)

		// Skip bridge devices (type 1) as they don't have sensor data
		if device.Type == 1 {
			log.Printf("Skipping bridge device %s", device.ID)
			continue
		}

		// Get current flow rate
		start = time.Now()
		flowRate, err := e.client.GetCurrentFlowRate(device.ID)
		duration = time.Since(start)

		if err != nil {
			log.Printf("Error getting flow rate for device %s: %v", device.ID, err)
			e.metrics.RecordScrapeMetrics("flow_rate", duration, false)
		} else {
			e.metrics.RecordScrapeMetrics("flow_rate", duration, true)
			// Use device ID as device name if location name is empty, otherwise use location name
			deviceName := device.Location.Name
			if deviceName == "" {
				deviceName = device.ID
			}
			e.metrics.UpdateCurrentFlowRate(device.ID, deviceName, device.Location.Name, flowRate.Value)
			log.Printf("Flow rate for device %s: %.2f %s", device.ID, flowRate.Value, flowRate.Units)
		}

		// Check if we should collect daily total water usage (twice per day + on start)
		if e.shouldCollectDailyTotalWaterUsage() {
			log.Printf("Collecting daily total water usage for device %s (scheduled collection)", device.ID)

			// Get daily total water usage for the last 30 days
			now := time.Now()
			thirtyDaysAgo := now.AddDate(0, 0, -30)
			startOfThirtyDaysAgo := time.Date(thirtyDaysAgo.Year(), thirtyDaysAgo.Month(), thirtyDaysAgo.Day(), 0, 0, 0, 0, now.Location())

			start = time.Now()
			dailyTotalUsage, err := e.client.QueryDailyTotalWaterUsage(device.ID, startOfThirtyDaysAgo, now)
			duration = time.Since(start)

			if err != nil {
				log.Printf("Error getting daily total water usage for device %s: %v", device.ID, err)
				e.metrics.RecordScrapeMetrics("daily_total_usage", duration, false)
			} else {
				e.metrics.RecordScrapeMetrics("daily_total_usage", duration, true)
				// Use device ID as device name if location name is empty, otherwise use location name
				deviceName := device.Location.Name
				if deviceName == "" {
					deviceName = device.ID
				}

				// Update daily total water usage metrics for each day
				for _, data := range dailyTotalUsage.Data {
					for _, dayData := range data.DailyTotalWaterUsage {
						// Extract date from datetime (format: "2025-08-01 00:00:00")
						date := dayData.DateTime[:10] // Get just the date part
						e.metrics.UpdateDailyTotalWaterUsage(device.ID, deviceName, device.Location.Name, date, dayData.Value)
					}
				}
				log.Printf("Updated daily total water usage for device %s with %d days of data", device.ID, len(dailyTotalUsage.Data))
			}
		} else {
			log.Printf("Skipping daily total water usage collection for device %s (not scheduled)", device.ID)
		}
	}

	log.Println("Metric collection completed")
}

// StartPeriodicCollection starts periodic metric collection
func (e *FlumeExporter) StartPeriodicCollection(interval time.Duration) {
	// Initial collection (authentication will happen automatically on first API call)
	e.CollectMetrics()

	// Start periodic collection
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			e.CollectMetrics()
		}
	}()
}
