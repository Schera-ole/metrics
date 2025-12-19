package models

const (
	Counter = "counter"
	Gauge   = "gauge"
)

type MetricsDTO struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

type Metric struct {
	Name  string
	Type  string
	Value any
}

type AuditEvent struct {
	TS        string   `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}
