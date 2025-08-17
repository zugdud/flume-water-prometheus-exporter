package main

import (
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the Flume exporter
type Metrics struct {
	// Current flow rate metrics
	currentFlowRate *prometheus.GaugeVec

	// Water usage metrics
	totalWaterUsage  *prometheus.GaugeVec
	hourlyWaterUsage *prometheus.GaugeVec
	dailyWaterUsage  *prometheus.GaugeVec

	// Device info metrics
	deviceInfo *prometheus.GaugeVec

	// Exporter metrics
	scrapeDuration *prometheus.GaugeVec
	scrapeSuccess  *prometheus.GaugeVec
	lastScrapeTime *prometheus.GaugeVec
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

		hourlyWaterUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_hourly_water_usage_gallons",
				Help: "Hourly water usage in gallons",
			},
			[]string{"device_id", "device_name", "location"},
		),

		dailyWaterUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "flume_daily_water_usage_gallons",
				Help: "Daily water usage in gallons",
			},
			[]string{"device_id", "device_name", "location"},
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
	}

	// Register all metrics
	prometheus.MustRegister(
		m.currentFlowRate,
		m.totalWaterUsage,
		m.hourlyWaterUsage,
		m.dailyWaterUsage,
		m.deviceInfo,
		m.scrapeDuration,
		m.scrapeSuccess,
		m.lastScrapeTime,
	)

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
		for _, queryData := range data.QueryData {
			if len(queryData) >= 2 {
				// queryData[1] should contain the usage value
				if usage, ok := queryData[1].(float64); ok {
					totalUsage += usage
				}
			}
		}

		// Update the appropriate metric based on bucket type
		switch bucket {
		case "HR":
			m.hourlyWaterUsage.WithLabelValues(deviceID, deviceName, location).Set(totalUsage)
		case "DAY":
			m.dailyWaterUsage.WithLabelValues(deviceID, deviceName, location).Set(totalUsage)
		}

		// Always update the total usage metric with bucket label
		m.totalWaterUsage.WithLabelValues(deviceID, deviceName, location, bucket).Set(totalUsage)
	}
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

// RecordScrapeMetrics records metrics about the scrape process
func (m *Metrics) RecordScrapeMetrics(endpoint string, duration time.Duration, success bool) {
	m.scrapeDuration.WithLabelValues(endpoint).Set(duration.Seconds())
	m.lastScrapeTime.WithLabelValues(endpoint).Set(float64(time.Now().Unix()))

	successValue := 0.0
	if success {
		successValue = 1.0
	}
	m.scrapeSuccess.WithLabelValues(endpoint).Set(successValue)
}

// FlumeExporter handles the collection of metrics from Flume API
type FlumeExporter struct {
	client  *FlumeClient
	metrics *Metrics
}

// NewFlumeExporter creates a new Flume exporter
func NewFlumeExporter(client *FlumeClient) *FlumeExporter {
	return &FlumeExporter{
		client:  client,
		metrics: NewMetrics(),
	}
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

	// Process each device
	for _, device := range devices {
		log.Printf("Processing device %s - Type: %d, Location: '%s'", device.ID, device.Type, device.Location.Name)

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

		// Get hourly usage for the last hour
		now := time.Now()
		hourAgo := now.Add(-1 * time.Hour)

		start = time.Now()
		hourlyUsage, err := e.client.QueryWaterUsage(device.ID, "HR", hourAgo, &now)
		duration = time.Since(start)

		if err != nil {
			log.Printf("Error getting hourly usage for device %s: %v", device.ID, err)
			e.metrics.RecordScrapeMetrics("hourly_usage", duration, false)
		} else {
			e.metrics.RecordScrapeMetrics("hourly_usage", duration, true)
			// Use device ID as device name if location name is empty, otherwise use location name
			deviceName := device.Location.Name
			if deviceName == "" {
				deviceName = device.ID
			}
			e.metrics.UpdateWaterUsage(device.ID, deviceName, device.Location.Name, hourlyUsage)
		}

		// Get daily usage for today
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		start = time.Now()
		dailyUsage, err := e.client.QueryWaterUsage(device.ID, "DAY", today, &now)
		duration = time.Since(start)

		if err != nil {
			log.Printf("Error getting daily usage for device %s: %v", device.ID, err)
			e.metrics.RecordScrapeMetrics("daily_usage", duration, false)
		} else {
			e.metrics.RecordScrapeMetrics("daily_usage", duration, true)
			// Use device ID as device name if location name is empty, otherwise use location name
			deviceName := device.Location.Name
			if deviceName == "" {
				deviceName = device.ID
			}
			e.metrics.UpdateWaterUsage(device.ID, deviceName, device.Location.Name, dailyUsage)
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
