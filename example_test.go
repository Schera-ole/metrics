package metrics_test

import (
	"context"
	"fmt"
	"testing"

	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
)

// Example of how to create and use metrics with the service layer
func Example_metricsService() {
	// Create a memory storage
	storage := repository.NewMemStorage()

	// Create a metrics service with the storage
	metricService := service.NewMetricsService(storage)

	ctx := context.Background()

	// Set a gauge metric
	gaugeVal := 3.14
	err := metricService.SetMetric(ctx, "Temperature", gaugeVal, models.Gauge)
	if err != nil {
		fmt.Printf("Error setting gauge metric: %v\n", err)
		return
	}

	// Set a counter metric
	counterVal := int64(42)
	err = metricService.SetMetric(ctx, "Requests", counterVal, models.Counter)
	if err != nil {
		fmt.Printf("Error setting counter metric: %v\n", err)
		return
	}

	// Retrieve a metric by name
	temp, err := metricService.GetMetricByName(ctx, "Temperature")
	if err != nil {
		fmt.Printf("Error getting metric: %v\n", err)
		return
	}

	fmt.Printf("Temperature: %v\n", temp)
	// Output: Temperature: 3.14
}

// Example of how to work with MetricsDTO
func Example_metricsDTO() {
	// Create a gauge metric DTO
	gaugeVal := 25.5
	gaugeMetric := models.MetricsDTO{
		ID:    "CPU_Temperature",
		MType: models.Gauge,
		Value: &gaugeVal,
	}

	// Create a counter metric DTO
	counterDelta := int64(100)
	counterMetric := models.MetricsDTO{
		ID:    "PageVisits",
		MType: models.Counter,
		Delta: &counterDelta,
	}

	fmt.Printf("Gauge: %s = %.1f (%s)\n", gaugeMetric.ID, *gaugeMetric.Value, gaugeMetric.MType)
	fmt.Printf("Counter: %s = %d (%s)\n", counterMetric.ID, *counterMetric.Delta, counterMetric.MType)
	// Output:
	// Gauge: CPU_Temperature = 25.5 (gauge)
	// Counter: PageVisits = 100 (counter)
}

// Simple test to demonstrate basic functionality
func TestExampleBasic(t *testing.T) {
	storage := repository.NewMemStorage()
	metricService := service.NewMetricsService(storage)

	ctx := context.Background()

	// Test setting a gauge
	val := 100.0
	err := metricService.SetMetric(ctx, "TestGauge", val, models.Gauge)
	if err != nil {
		t.Fatalf("Failed to set gauge: %v", err)
	}

	// Test retrieving the gauge
	result, err := metricService.GetMetricByName(ctx, "TestGauge")
	if err != nil {
		t.Fatalf("Failed to get gauge: %v", err)
	}

	if result != val {
		t.Errorf("Expected %f, got %v", val, result)
	}
}
