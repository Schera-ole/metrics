package repository

import (
	"context"

	models "github.com/Schera-ole/metrics/internal/model"
)

type Repository interface {
	SetMetric(ctx context.Context, name string, value any, typ string) error
	SetMetrics(ctx context.Context, metrics []models.Metric) error
	GetMetric(ctx context.Context, metrics models.MetricsDTO) (any, error)
	GetMetricByName(ctx context.Context, name string) (any, error)
	DeleteMetric(ctx context.Context, name string) error
	ListMetrics(ctx context.Context) ([]models.Metric, error)
	Ping(ctx context.Context) error
	Close() error
}
