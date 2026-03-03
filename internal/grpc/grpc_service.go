package grpcserver

import (
	"context"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	grpcMetrics "github.com/Schera-ole/metrics/internal/proto"
	metricsService "github.com/Schera-ole/metrics/internal/service"
)

// MetricsServer implements the gRPC Metrics service.
type MetricsServer struct {
	grpcMetrics.UnimplementedMetricsServer
	// metricsService is used to store received metrics
	metricsService *metricsService.MetricsService
}

// NewMetricsServer creates a new gRPC server instance with the given metrics service.
func NewMetricsServer(metricsService *metricsService.MetricsService) *MetricsServer {
	return &MetricsServer{
		metricsService: metricsService,
	}
}

// UpdateMetrics processes a batch of metrics received from the agent.
func (s *MetricsServer) UpdateMetrics(ctx context.Context, in *grpcMetrics.UpdateMetricsRequest) (*grpcMetrics.UpdateMetricsResponse, error) {
	// Convert proto metrics to internal model
	var metrics []models.Metric
	for _, m := range in.GetMetrics() {
		metric := models.Metric{
			Name: m.GetId(),
			Type: getInternalMetricType(m.GetType()),
		}

		// Set value based on metric type
		if m.GetType() == grpcMetrics.Metric_GAUGE {
			metric.Value = m.GetValue()
		} else if m.GetType() == grpcMetrics.Metric_COUNTER {
			metric.Value = m.GetDelta()
		}

		metrics = append(metrics, metric)
	}

	// Use existing MetricsService to store metrics
	err := s.metricsService.SetMetrics(ctx, metrics)
	if err != nil {
		return nil, err
	}

	return &grpcMetrics.UpdateMetricsResponse{}, nil
}

// getInternalMetricType converts gRPC MType enum to internal type string.
func getInternalMetricType(typ grpcMetrics.Metric_MType) string {
	switch typ {
	case grpcMetrics.Metric_GAUGE:
		return config.GaugeType
	case grpcMetrics.Metric_COUNTER:
		return config.CounterType
	default:
		return ""
	}
}
