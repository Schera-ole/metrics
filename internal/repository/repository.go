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
	GetMetricWithModels(metrics models.MetricsDTO) (any, error)
	GetMetric(name string) (any, error)
	DeleteMetric(name string) error
	ListMetrics() []struct {
		Name  string
		Value any
	}
	ExportMetrics() []models.Metric
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
		types:    make(map[string]string),
	}
}

func (ms *MemStorage) SetMetric(name string, value any, typ string) error {
	switch typ {
	case config.CounterType:
		val := value.(int64)
		_, exists := ms.counters[name]
		if exists {
			ms.counters[name] += val
		} else {
			ms.counters[name] = val
			ms.types[name] = typ
		}
	case config.GaugeType:
		val := value.(float64)
		ms.gauges[name] = val
		ms.types[name] = typ
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

func (ms *MemStorage) GetMetricWithModels(metrics models.MetricsDTO) (any, error) {
	metricType, exists := ms.types[metrics.ID]
	if !exists {
		return nil, errors.New("metric is not found")
	}

	// Create a new metrics struct for the response
	responseMetrics := models.MetricsDTO{
		ID:    metrics.ID,
		MType: metricType,
	}

	switch metricType {
	case config.GaugeType:
		if val, exists := ms.gauges[metrics.ID]; exists {
			responseMetrics.Value = &val
		}
	case config.CounterType:
		if val, exists := ms.counters[metrics.ID]; exists {
			responseMetrics.Delta = &val
		}
	default:
		return nil, errors.New("unknown type of metric")
	}
	return responseMetrics, nil
}

func (ms *MemStorage) GetMetric(name string) (any, error) {
	metricType, exists := ms.types[name]
	if !exists {
		return nil, errors.New("metric is not found")
	}
	switch metricType {
	case config.GaugeType:
		return ms.gauges[name], nil
	case config.CounterType:
		return ms.counters[name], nil
	default:
		return nil, errors.New("unknown type of metric")
	}
}

func (ms *MemStorage) ExportMetrics() []models.Metric {
	metrics := ms.ListMetrics()

	var formattedMetrics []models.Metric
	for _, m := range metrics {
		typ := ms.types[m.Name]
		formattedMetrics = append(formattedMetrics, models.Metric{
			Name:  m.Name,
			Type:  typ,
			Value: m.Value,
		})
	}
	return formattedMetrics
}
