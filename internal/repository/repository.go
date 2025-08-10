package repository

import (
	"errors"

	"github.com/Schera-ole/metrics/internal/config"
)

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	types    map[string]string
}

type Repository interface {
	SetMetric(name string, value interface{}, typ string) error
	GetMetric(name string) (interface{}, error)
	DeleteMetric(name string) error
	ListMetrics() []string
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
		types:    make(map[string]string),
	}
}

func (ms *MemStorage) SetMetric(name string, value interface{}, typ string) error {
	switch value := value.(type) {
	case float64:
		ms.gauges[name] = value
		ms.types[name] = typ
	case int64:
		_, exists := ms.counters[name]
		if exists {
			ms.counters[name] += value
		} else {
			ms.counters[name] = value
			ms.types[name] = typ
		}
	}
	return nil
}

func (ms *MemStorage) DeleteMetric(name string) error {
	delete(ms.gauges, name)
	delete(ms.counters, name)
	delete(ms.types, name)
	return nil
}

func (ms *MemStorage) ListMetrics() []string {
	var metrics []string
	for k := range ms.types {
		metrics = append(metrics, k)
	}
	return metrics
}

func (ms *MemStorage) GetMetric(name string) (interface{}, error) {
	metricType, exists := ms.types[name]
	if !exists {
		return nil, errors.New("Metric is not found")
	}
	switch metricType {
	case config.GaugeType:
		return ms.gauges[name], nil
	case config.CounterType:
		return ms.counters[name], nil
	default:
		return nil, errors.New("Unknown type of metric")
	}
}
