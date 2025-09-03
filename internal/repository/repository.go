package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"go.uber.org/zap"
)

type Metric struct {
	Name  string
	Type  string
	Value any
}

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	types    map[string]string
}

type Repository interface {
	SetMetric(name string, value any, typ string) error
	GetMetricWithModels(metrics models.Metrics) (any, error)
	GetMetric(name string) (any, error)
	DeleteMetric(name string) error
	ListMetrics() []struct {
		Name  string
		Value any
	}
	RestoreMetrics(fname string, logger *zap.SugaredLogger) error
	SaveMetrics(fname string) error
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

func (ms *MemStorage) GetMetricWithModels(metrics models.Metrics) (any, error) {
	metricType, exists := ms.types[metrics.ID]
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

func (ms *MemStorage) SaveMetrics(fname string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(fname)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	file, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	metrics := ms.ListMetrics()

	// Convert to the format we need for saving
	var formattedMetrics []Metric
	for _, m := range metrics {
		typ := ms.types[m.Name]
		formattedMetrics = append(formattedMetrics, Metric{
			Name:  m.Name,
			Type:  typ,
			Value: m.Value,
		})
	}

	return encoder.Encode(formattedMetrics)
}

func (ms *MemStorage) RestoreMetrics(fname string, logger *zap.SugaredLogger) error {
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		logger.Infof("storage file not exists %s", fname)
		return nil
	}

	file, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("error while opening file to restore: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	metrics := []Metric{}
	err = decoder.Decode(&metrics)
	if err != nil {
		return fmt.Errorf("error while marshalling file store: %w", err)
	}
	for _, metric := range metrics {
		ms.SetMetric(metric.Name, metric.Value, metric.Type)
	}
	return nil
}
