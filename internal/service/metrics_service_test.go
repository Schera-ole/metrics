package service

import (
	"context"
	"os"
	"testing"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewMetricsService(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	assert.NotNil(t, service)
	assert.Equal(t, memStorage, service.repository)
}

func TestMetricsService_SetMetric(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Test setting a gauge metric
	err := service.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	// Verify the metric was set
	value, err := service.GetMetricByName(ctx, "testGauge")
	require.NoError(t, err)
	assert.Equal(t, 42.5, value)
}

func TestMetricsService_SetMetrics(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Prepare test data
	metrics := []models.Metric{
		{Name: "gauge1", Type: config.GaugeType, Value: 1.5},
		{Name: "counter1", Type: config.CounterType, Value: int64(10)},
	}

	// Set metrics
	err := service.SetMetrics(ctx, metrics)
	require.NoError(t, err)

	// Verify the metrics were set
	value1, err := service.GetMetricByName(ctx, "gauge1")
	require.NoError(t, err)
	assert.Equal(t, 1.5, value1)

	value2, err := service.GetMetricByName(ctx, "counter1")
	require.NoError(t, err)
	assert.Equal(t, int64(10), value2)
}

func TestMetricsService_GetMetric(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Set up a test metric
	err := memStorage.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	// Prepare input DTO
	inputDTO := models.MetricsDTO{
		ID:    "testGauge",
		MType: config.GaugeType,
	}

	// Get the metric
	result, err := service.GetMetric(ctx, inputDTO)
	require.NoError(t, err)

	// Check that we got the expected value
	assert.Equal(t, "testGauge", result.ID)
	assert.Equal(t, config.GaugeType, result.MType)
	assert.NotNil(t, result.Value)
	assert.Equal(t, 42.5, *result.Value)
}

func TestMetricsService_GetMetricByName(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Set up a test metric
	err := memStorage.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	// Get the metric by name
	result, err := service.GetMetricByName(ctx, "testGauge")
	require.NoError(t, err)
	assert.Equal(t, 42.5, result)
}

func TestMetricsService_DeleteMetric(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Set up a test metric
	err := memStorage.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	// Delete the metric
	err = service.DeleteMetric(ctx, "testGauge")
	require.NoError(t, err)

	// Try to get the deleted metric (should fail)
	_, err = service.GetMetricByName(ctx, "testGauge")
	assert.Error(t, err)
}

func TestMetricsService_ListMetrics(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Set up test metrics
	err := memStorage.SetMetric(ctx, "gauge1", 1.5, config.GaugeType)
	require.NoError(t, err)
	err = memStorage.SetMetric(ctx, "counter1", int64(10), config.CounterType)
	require.NoError(t, err)

	// List all metrics
	result, err := service.ListMetrics(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Check that we got the expected metrics
	foundGauge := false
	foundCounter := false
	for _, metric := range result {
		if metric.Name == "gauge1" && metric.Type == config.GaugeType && metric.Value == 1.5 {
			foundGauge = true
		}
		if metric.Name == "counter1" && metric.Type == config.CounterType && metric.Value == int64(10) {
			foundCounter = true
		}
	}
	assert.True(t, foundGauge)
	assert.True(t, foundCounter)
}

func TestMetricsService_Ping(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Ping should succeed with memstorage
	err := service.Ping(ctx)
	require.NoError(t, err)
}

func TestMetricsService_IsMemStorage(t *testing.T) {
	// Test with MemStorage
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	assert.True(t, service.IsMemStorage())
}

func TestMetricsService_SaveMetrics(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Add a metric to save
	err := memStorage.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	// Save metrics to a file
	filename := "/tmp/test_metrics.json"
	err = service.SaveMetrics(ctx, filename)
	require.NoError(t, err)

	// Check that the file was created
	_, err = os.Stat(filename)
	assert.NoError(t, err)

	// Clean up
	os.Remove(filename)
}

func TestMetricsService_RestoreMetrics(t *testing.T) {
	memStorage := repository.NewMemStorage()
	service := NewMetricsService(memStorage)
	ctx := context.Background()

	// Set up logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	logSugar := logger.Sugar()

	// Test restoring from non-existent file
	filename := "/tmp/non_existent_file.json"
	err := service.RestoreMetrics(ctx, filename, logSugar)
	require.NoError(t, err)

	// Test restoring from an existing file with valid JSON
	filename2 := "/tmp/test_metrics_restore.json"
	content := `[{"Name":"testGauge","Type":"gauge","Value":42.5},{"Name":"testCounter","Type":"counter","Value":10}]`
	err = os.WriteFile(filename2, []byte(content), 0644)
	require.NoError(t, err)

	err = service.RestoreMetrics(ctx, filename2, logSugar)
	require.NoError(t, err)

	// Verify metrics were restored
	value1, err := service.GetMetricByName(ctx, "testGauge")
	require.NoError(t, err)
	assert.Equal(t, 42.5, value1)

	value2, err := service.GetMetricByName(ctx, "testCounter")
	require.NoError(t, err)
	assert.Equal(t, int64(10), value2)

	// Clean up
	os.Remove(filename2)
}
