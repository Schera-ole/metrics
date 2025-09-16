package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"go.uber.org/zap"
)

type MetricsService struct {
	repository repository.Repository
}

func NewMetricsService(repo repository.Repository) *MetricsService {
	return &MetricsService{repository: repo}
}

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
