// Package models defines the data structures used throughout the metrics system.
package models

const (
	Counter = "counter"
	Gauge   = "gauge"
)

// MetricsDTO represents a metric data transfer object for API requests and responses.
type MetricsDTO struct {
	// ID is the unique identifier for the metric
	ID string `json:"id"`

	// MType is the type of the metric (either "counter" or "gauge")
	MType string `json:"type"`

	// Delta is the increment value for counter metrics (omitted for gauge metrics)
	Delta *int64 `json:"delta,omitempty"`

	// Value is the value for gauge metrics (omitted for counter metrics)
	Value *float64 `json:"value,omitempty"`
}

// Metric represents a single metric with its name, type, and value.
type Metric struct {
	// Name is the unique identifier for the metric
	Name string

	// Type is the type of the metric (either "counter" or "gauge")
	Type string

	// Value is the metric value (int64 for counters, float64 for gauges)
	Value any
}

// AuditEvent represents an audit log entry for metric operations.
type AuditEvent struct {
	// TS is the timestamp of the event in ISO 8601 format
	TS string `json:"ts"`

	// Metrics is a list of metric names affected by the operation
	Metrics []string `json:"metrics"`

	// IPAddress is the IP address of the client that initiated the operation
	IPAddress string `json:"ip_address"`
}
