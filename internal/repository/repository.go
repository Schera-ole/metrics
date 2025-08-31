package repository

import (
	"errors"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
)

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	types    map[string]string
}

type Repository interface {
	SetMetric(name string, value any, typ string) error
	GetMetric(metrics models.Metrics) (any, error)
	DeleteMetric(name string) error
	ListMetrics() []struct {
		Name  string
		Value any
	}
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
		types:    make(map[string]string),
	}
}

func (ms *MemStorage) SetMetric(name string, value any, typ string) error {
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

func (ms *MemStorage) ListMetrics() []struct {
	Name  string
	Value any
} {
	result := make([]struct {
		Name  string
		Value any
	}, 0)

	for name, typ := range ms.types {
		var value any

		switch typ {
		case config.GaugeType:
			value = ms.gauges[name]
		case config.CounterType:
			value = ms.counters[name]
		default:
			continue
		}

		result = append(result, struct {
			Name  string
			Value any
		}{Name: name, Value: value})
	}
	return result
}

func (ms *MemStorage) GetMetric(metrics models.Metrics) (any, error) {
	metricType, exists := ms.types[metrics.MType]
	if !exists {
		return nil, errors.New("metric is not found")
	}
	switch metricType {
	case config.GaugeType:
		if val, exists := ms.gauges[metrics.ID]; exists {
			metrics.Value = &val
		}
	case config.CounterType:
		if val, exists := ms.counters[metrics.ID]; exists {
			metrics.Delta = &val
		}
	default:
		return nil, errors.New("unknown type of metric")
	}
	return metrics, nil
}
