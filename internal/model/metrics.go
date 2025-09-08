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
