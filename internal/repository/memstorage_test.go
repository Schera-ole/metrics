package repository

import (
	"context"
	"testing"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemStorage(t *testing.T) {
	storage := NewMemStorage()
	assert.NotNil(t, storage)
	assert.NotNil(t, storage.gauges)
	assert.NotNil(t, storage.counters)
	assert.NotNil(t, storage.types)
}

func TestMemStorage_SetAndGetMetric(t *testing.T) {
	storage := NewMemStorage()
	ctx := context.Background()

	// Test setting and getting a gauge metric
	err := storage.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	val, err := storage.GetMetricByName(ctx, "testGauge")
	require.NoError(t, err)
	assert.Equal(t, 42.5, val)

	// Test setting and getting a counter metric
	err = storage.SetMetric(ctx, "testCounter", int64(10), config.CounterType)
	require.NoError(t, err)

	val, err = storage.GetMetricByName(ctx, "testCounter")
	require.NoError(t, err)
	assert.Equal(t, int64(10), val)

	// Test getting a non-existent metric
	_, err = storage.GetMetricByName(ctx, "nonExistent")
	assert.Error(t, err)
}

func TestMemStorage_SetMetricIncrementCounter(t *testing.T) {
	storage := NewMemStorage()
	ctx := context.Background()

	// Set initial counter value
	err := storage.SetMetric(ctx, "incrementCounter", int64(5), config.CounterType)
	require.NoError(t, err)

	// Increment the counter
	err = storage.SetMetric(ctx, "incrementCounter", int64(3), config.CounterType)
	require.NoError(t, err)

	// Check that the counter was incremented
	val, err := storage.GetMetricByName(ctx, "incrementCounter")
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)
}

func TestMemStorage_DeleteMetric(t *testing.T) {
	storage := NewMemStorage()
	ctx := context.Background()

	// Add a gauge metric
	err := storage.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	// Verify it exists
	_, err = storage.GetMetricByName(ctx, "testGauge")
	require.NoError(t, err)

	// Delete the metric
	err = storage.DeleteMetric(ctx, "testGauge")
	require.NoError(t, err)

	// Verify it's deleted
	_, err = storage.GetMetricByName(ctx, "testGauge")
	assert.Error(t, err)
}

func TestMemStorage_ListMetrics(t *testing.T) {
	storage := NewMemStorage()
	ctx := context.Background()

	// Add some metrics
	err := storage.SetMetric(ctx, "gauge1", 1.5, config.GaugeType)
	require.NoError(t, err)

	err = storage.SetMetric(ctx, "counter1", int64(10), config.CounterType)
	require.NoError(t, err)

	// List all metrics
	metrics, err := storage.ListMetrics(ctx)
	require.NoError(t, err)
	assert.Len(t, metrics, 2)

	// Check that both metrics are in the list
	foundGauge := false
	foundCounter := false
	for _, metric := range metrics {
		if metric.Name == "gauge1" && metric.Type == config.GaugeType {
			assert.Equal(t, 1.5, metric.Value)
			foundGauge = true
		}
		if metric.Name == "counter1" && metric.Type == config.CounterType {
			assert.Equal(t, int64(10), metric.Value)
			foundCounter = true
		}
	}
	assert.True(t, foundGauge)
	assert.True(t, foundCounter)
}

func TestMemStorage_GetMetric(t *testing.T) {
	storage := NewMemStorage()
	ctx := context.Background()

	// Add a gauge metric
	err := storage.SetMetric(ctx, "testGauge", 42.5, config.GaugeType)
	require.NoError(t, err)

	// Create a metrics DTO to query
	value := 42.5
	dto := models.MetricsDTO{
		ID:    "testGauge",
		MType: config.GaugeType,
		Value: &value,
	}

	// Get the metric
	result, err := storage.GetMetric(ctx, dto)
	require.NoError(t, err)
	assert.Equal(t, "testGauge", result.ID)
	assert.Equal(t, config.GaugeType, result.MType)
	assert.NotNil(t, result.Value)
	assert.Equal(t, 42.5, *result.Value)

	// Try to get a non-existent metric
	dtoNonExistent := models.MetricsDTO{
		ID:    "nonExistent",
		MType: config.GaugeType,
	}

	_, err = storage.GetMetric(ctx, dtoNonExistent)
	assert.Error(t, err)
}

func TestMemStorage_Ping(t *testing.T) {
	storage := NewMemStorage()
	ctx := context.Background()

	// Ping should always succeed for MemStorage
	err := storage.Ping(ctx)
	assert.NoError(t, err)
}

func TestMemStorage_Close(t *testing.T) {
	storage := NewMemStorage()

	// Close should always succeed for MemStorage
	err := storage.Close()
	assert.NoError(t, err)
}

func TestMemStorage_SetMetrics(t *testing.T) {
	storage := NewMemStorage()
	ctx := context.Background()

	// Prepare batch of metrics
	metrics := []models.Metric{
		{Name: "batchGauge", Type: config.GaugeType, Value: 3.14},
		{Name: "batchCounter", Type: config.CounterType, Value: int64(42)},
	}

	// Set metrics in batch
	err := storage.SetMetrics(ctx, metrics)
	require.NoError(t, err)

	// Verify gauge was set
	val, err := storage.GetMetricByName(ctx, "batchGauge")
	require.NoError(t, err)
	assert.Equal(t, 3.14, val)

	// Verify counter was set
	val, err = storage.GetMetricByName(ctx, "batchCounter")
	require.NoError(t, err)
	assert.Equal(t, int64(42), val)
}
