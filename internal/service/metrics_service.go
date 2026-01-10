// Package service provides the business logic layer for the metrics system.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
)

// MetricsService provides methods for managing metrics.
//
// It delegates operations to an underlying repository implementation.
type MetricsService struct {
	// repository is the underlying data storage implementation
	repository repository.Repository
}

// NewMetricsService creates a new MetricsService with the specified repository.
func NewMetricsService(repo repository.Repository) *MetricsService {

	return &MetricsService{repository: repo}
}

// SetMetric sets a single metric value, delegating to the repository implementation.
func (ms *MetricsService) SetMetric(ctx context.Context, name string, value any, typ string) error {

	return ms.repository.SetMetric(ctx, name, value, typ)
}

// SetMetrics sets multiple metrics in a batch operation, delegating to the repository implementation.
func (ms *MetricsService) SetMetrics(ctx context.Context, metrics []models.Metric) error {

	return ms.repository.SetMetrics(ctx, metrics)
}

// GetMetric retrieves a single metric by its DTO, delegating to the repository implementation.
func (ms *MetricsService) GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error) {

	return ms.repository.GetMetric(ctx, metrics)
}

// GetMetricByName retrieves a single metric by its name, delegating to the repository implementation.
func (ms *MetricsService) GetMetricByName(ctx context.Context, name string) (any, error) {

	return ms.repository.GetMetricByName(ctx, name)
}

// DeleteMetric removes a metric by its name, delegating to the repository implementation.
func (ms *MetricsService) DeleteMetric(ctx context.Context, name string) error {

	return ms.repository.DeleteMetric(ctx, name)
}

// ListMetrics retrieves all metrics, delegating to the repository implementation.
func (ms *MetricsService) ListMetrics(ctx context.Context) ([]models.Metric, error) {

	return ms.repository.ListMetrics(ctx)
}

// Ping checks the repository connection, delegating to the repository implementation.
func (ms *MetricsService) Ping(ctx context.Context) error {

	return ms.repository.Ping(ctx)
}

// IsMemStorage checks if the underlying repository is a MemStorage implementation.
func (ms *MetricsService) IsMemStorage() bool {

	_, isMemStorage := ms.repository.(*repository.MemStorage)
	return isMemStorage
}

// SaveMetrics saves all metrics to a file in JSON format.
func (ms *MetricsService) SaveMetrics(ctx context.Context, fname string) error {

	file, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	metrics, _ := ms.repository.ListMetrics(ctx)

	return encoder.Encode(metrics)
}

// RestoreMetrics restores metrics from a file.
//
// It reads metrics from the specified file and stores them in the repository.
func (ms *MetricsService) RestoreMetrics(ctx context.Context, fname string, logger *zap.SugaredLogger) error {

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
	metrics := []models.Metric{}
	err = decoder.Decode(&metrics)
	if err != nil {
		return fmt.Errorf("error while marshalling file store: %w", err)
	}

	for _, metric := range metrics {
		value := metric.Value
		if metric.Type == config.CounterType {
			if floatValue, ok := metric.Value.(float64); ok {
				value = int64(floatValue)
			}
		}
		ms.repository.SetMetric(ctx, metric.Name, value, metric.Type)
	}
	return nil
}
