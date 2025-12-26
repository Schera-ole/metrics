// Package repository provides data storage interfaces and implementations for the metrics system.
package repository

import (
	"context"

	models "github.com/Schera-ole/metrics/internal/model"
)

// Repository defines the interface for metrics storage implementations.
type Repository interface {
	// SetMetric stores a single metric value
	SetMetric(ctx context.Context, name string, value any, typ string) error

	// SetMetrics stores multiple metrics in a batch operation
	SetMetrics(ctx context.Context, metrics []models.Metric) error

	// GetMetric retrieves a single metric by its DTO
	GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error)

	// GetMetricByName retrieves a single metric by its name
	GetMetricByName(ctx context.Context, name string) (any, error)

	// DeleteMetric removes a metric by its name
	DeleteMetric(ctx context.Context, name string) error

	// ListMetrics retrieves all metrics
	ListMetrics(ctx context.Context) ([]models.Metric, error)

	// Ping checks the repository connection
	Ping(ctx context.Context) error

	// Close releases any resources held by the repository
	Close() error
}
