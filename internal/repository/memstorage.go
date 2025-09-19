package repository

import (
	"context"
	"errors"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
)

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	types    map[string]string
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
		types:    make(map[string]string),
	}
}

func (ms *MemStorage) SetMetric(ctx context.Context, name string, value any, typ string) error {
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

func (ms *MemStorage) DeleteMetric(ctx context.Context, name string) error {
	delete(ms.gauges, name)
	delete(ms.counters, name)
	delete(ms.types, name)
	return nil
}

func (ms *MemStorage) ListMetrics(ctx context.Context) ([]models.Metric, error) {
	var result []models.Metric

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
			Type  string
			Value any
		}{Name: name, Type: typ, Value: value})
	}
	return result, nil
}

func (ms *MemStorage) GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error) {
	metricType, exists := ms.types[metrics.ID]
	if !exists {
		return models.MetricsDTO{}, errors.New("metric is not found")
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
		return models.MetricsDTO{}, errors.New("unknown type of metric")
	}
	return responseMetrics, nil
}

func (ms *MemStorage) GetMetricByName(ctx context.Context, name string) (any, error) {
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

func (ms *MemStorage) Close() error {
	return nil
}

func (ms *MemStorage) Ping(ctx context.Context) error {
	return nil
}

func (ms *MemStorage) SetMetrics(ctx context.Context, metrics []models.Metric) error {
	for _, metric := range metrics {
		switch metric.Type {
		case config.CounterType:
			val := metric.Value.(int64)
			_, exists := ms.counters[metric.Name]
			if exists {
				ms.counters[metric.Name] += val
			} else {
				ms.counters[metric.Name] = val
				ms.types[metric.Name] = metric.Type
			}
		case config.GaugeType:
			val := metric.Value.(float64)
			ms.gauges[metric.Name] = val
			ms.types[metric.Name] = metric.Type
		}
	}
	return nil
}
