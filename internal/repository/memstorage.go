package repository

import (
	"context"
	"sync"

	"github.com/Schera-ole/metrics/internal/config"
	internalerrors "github.com/Schera-ole/metrics/internal/errors"
	models "github.com/Schera-ole/metrics/internal/model"
)

// MemStorage implements the Repository interface using in-memory storage.
type MemStorage struct {
	// mu provides thread-safe access to the storage maps
	mu sync.RWMutex

	// gauges stores gauge metrics as name -> value pairs
	gauges map[string]float64

	// counters stores counter metrics as name -> value pairs
	counters map[string]int64

	// types stores the metric type for each metric name
	types map[string]string
}

// NewMemStorage creates a new in-memory storage instance.
//
// It initializes empty maps for gauges, counters, and metric types.
func NewMemStorage() *MemStorage {

	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
		types:    make(map[string]string),
	}
}

// SetMetric stores a single metric value in memory.
//
// For counters, it adds the value to the existing counter (or creates a new one).
// For gauges, it replaces the existing value (or creates a new one).
func (ms *MemStorage) SetMetric(ctx context.Context, name string, value any, typ string) error {

	ms.mu.Lock()
	defer ms.mu.Unlock()
	switch typ {
	case config.CounterType:
		val := value.(int64)
		_, exists := ms.counters[name]
		if exists {
			ms.counters[name] += val
		} else {
			ms.counters[name] = val
		}
		ms.types[name] = typ
	case config.GaugeType:
		val := value.(float64)
		ms.gauges[name] = val
		ms.types[name] = typ
	}
	return nil
}

// DeleteMetric removes a metric from memory storage.
//
// It deletes the metric from all maps (gauges, counters, and types).
func (ms *MemStorage) DeleteMetric(ctx context.Context, name string) error {

	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.gauges, name)
	delete(ms.counters, name)
	delete(ms.types, name)
	return nil
}

// ListMetrics returns all metrics stored in memory.
//
// It creates a slice of Metric structs containing all gauge and counter values.
func (ms *MemStorage) ListMetrics(ctx context.Context) ([]models.Metric, error) {

	ms.mu.RLock()
	defer ms.mu.RUnlock()
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

// GetMetric retrieves a single metric by its DTO.
//
// It returns a MetricsDTO with the current value of the requested metric.
func (ms *MemStorage) GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error) {

	ms.mu.RLock()
	defer ms.mu.RUnlock()
	metricType, exists := ms.types[metrics.ID]
	if !exists {
		return models.MetricsDTO{}, internalerrors.ErrMetricNotFound
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
		return models.MetricsDTO{}, internalerrors.ErrUnknownMetricType
	}
	return responseMetrics, nil
}

// GetMetricByName retrieves a single metric by its name.
//
// It returns the raw value of the requested metric (float64 for gauges, int64 for counters).
func (ms *MemStorage) GetMetricByName(ctx context.Context, name string) (any, error) {

	ms.mu.RLock()
	defer ms.mu.RUnlock()
	metricType, exists := ms.types[name]
	if !exists {
		return nil, internalerrors.ErrMetricNotFound
	}
	switch metricType {
	case config.GaugeType:
		return ms.gauges[name], nil
	case config.CounterType:
		return ms.counters[name], nil
	default:
		return nil, internalerrors.ErrUnknownMetricType
	}
}

// Close releases any resources held by the memory storage.
func (ms *MemStorage) Close() error {

	return nil
}

// Ping checks the health of the memory storage.
//
// For MemStorage, this always returns nil since there are no external dependencies.
func (ms *MemStorage) Ping(ctx context.Context) error {
	return nil
}

// SetMetrics stores multiple metrics in memory.
//
// It processes a slice of Metric structs, setting each one according to its type.
func (ms *MemStorage) SetMetrics(ctx context.Context, metrics []models.Metric) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, metric := range metrics {
		switch metric.Type {
		case config.CounterType:
			val := metric.Value.(int64)
			_, exists := ms.counters[metric.Name]
			if exists {
				ms.counters[metric.Name] += val
			} else {
				ms.counters[metric.Name] = val
			}
			ms.types[metric.Name] = metric.Type
		case config.GaugeType:
			val := metric.Value.(float64)
			ms.gauges[metric.Name] = val
			ms.types[metric.Name] = metric.Type
		}
	}
	return nil
}
