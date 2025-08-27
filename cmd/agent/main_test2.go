package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"testing"

	"github.com/Schera-ole/metrics/internal/agent"
	"github.com/Schera-ole/metrics/internal/config"
)

func TestCollectMetrics(t *testing.T) {
	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)

	expectedMetrics := []agent.Metric{
		{Name: "Alloc", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("Alloc").Interface().(uint64))},
		{Name: "TotalAlloc", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("TotalAlloc").Interface().(uint64))},
		{Name: "Sys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("Sys").Interface().(uint64))},
		{Name: "Lookups", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("Lookups").Interface().(uint64))},
		{Name: "Mallocs", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("Mallocs").Interface().(uint64))},
		{Name: "Frees", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("Frees").Interface().(uint64))},
		{Name: "HeapAlloc", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("HeapAlloc").Interface().(uint64))},
		{Name: "HeapSys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("HeapSys").Interface().(uint64))},
		{Name: "HeapIdle", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("HeapIdle").Interface().(uint64))},
		{Name: "HeapInuse", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("HeapInuse").Interface().(uint64))},
		{Name: "HeapReleased", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("HeapReleased").Interface().(uint64))},
		{Name: "HeapObjects", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("HeapObjects").Interface().(uint64))},
		{Name: "StackInuse", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("StackInuse").Interface().(uint64))},
		{Name: "StackSys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("StackSys").Interface().(uint64))},
		{Name: "MSpanInuse", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("MSpanInuse").Interface().(uint64))},
		{Name: "MSpanSys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("MSpanSys").Interface().(uint64))},
		{Name: "MCacheInuse", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("MCacheInuse").Interface().(uint64))},
		{Name: "MCacheSys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("MCacheSys").Interface().(uint64))},
		{Name: "BuckHashSys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("BuckHashSys").Interface().(uint64))},
		{Name: "GCSys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("GCSys").Interface().(uint64))},
		{Name: "OtherSys", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("OtherSys").Interface().(uint64))},
		{Name: "NextGC", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("NextGC").Interface().(uint64))},
		{Name: "LastGC", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("LastGC").Interface().(int64))},
		{Name: "PauseTotalNs", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("PauseTotalNs").Interface().(uint64))},
		{Name: "NumGC", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("NumGC").Interface().(uint32))},
		{Name: "NumForcedGC", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("NumForcedGC").Interface().(uint32))},
		{Name: "GCCPUFraction", Type: config.GaugeType, Value: float64(reflect.ValueOf(new(runtime.MemStats)).FieldByName("GCCPUFraction").Interface().(float64))},
		{Name: "EnableGC", Type: config.GaugeType, Value: boolToInt(reflect.ValueOf(new(runtime.MemStats)).FieldByName("EnableGC").Interface().(bool))},
		{Name: "DebugGC", Type: config.GaugeType, Value: boolToInt(reflect.ValueOf(new(runtime.MemStats)).FieldByName("DebugGC").Interface().(bool))},
		{Name: "RandomValue", Type: config.GaugeType, Value: counter.Value + 1},
		{Name: "PollCount", Type: config.CounterType, Value: counter.Value + 1},
	}

	if len(metrics) != len(expectedMetrics) {
		t.Fatalf("Expected %d metrics, got %d", len(expectedMetrics), len(metrics))
	}

	for i, metric := range metrics {
		if metric.Name != expectedMetrics[i].Name || metric.Type != expectedMetrics[i].Type {
			t.Errorf("Metric %d: expected %v, got %v", i, expectedMetrics[i], metric)
		}
		// RandomValue and PollCount are dynamic, so we check their types and values separately
		if metric.Name == "RandomValue" {
			if metric.Type != config.GaugeType {
				t.Errorf("RandomValue: expected type %v, got %v", config.GaugeType, metric.Type)
			}
		} else if metric.Name == "PollCount" {
			if metric.Type != config.CounterType {
				t.Errorf("PollCount: expected type %v, got %v", config.CounterType, metric.Type)
			}
			if metric.Value.(int64) != counter.Value+1 {
				t.Errorf("PollCount: expected value %d, got %d", counter.Value+1, metric.Value.(int64))
			}
		}
	}
}

func TestSendMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedURL := "/update/gauge/Alloc/100"
		if r.URL.Path != expectedURL {
			t.Errorf("Expected URL %s, got %s", expectedURL, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	metrics := []agent.Metric{
		{Name: "Alloc", Type: config.GaugeType, Value: 100},
	}

	err := sendMetrics(metrics, ts.URL+"/update")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
