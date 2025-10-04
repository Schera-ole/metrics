package main

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectMetrics(t *testing.T) {
	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)
	require.NotEmpty(t, metrics)

	foundPollCount := false
	for _, m := range metrics {
		t.Logf("Checking metric: Name='%s', Type='%s', Value='%v'", m.Name, m.Type, m.Value)
		assert.NotEmpty(t, m.Name)

		if m.Name == "PollCount" {
			foundPollCount = true
			assert.Equal(t, "counter", m.Type)
			assert.Equal(t, int64(1), m.Value)
		} else {
			assert.Equal(t, "gauge", m.Type)
		}
	}

	assert.True(t, foundPollCount, "PollCount metric should be present")
}

func TestSendMetric(t *testing.T) {
	var receivedMetrics []models.MetricsDTO
	var key string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))

		gzipReader, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		defer gzipReader.Close()

		body, err := io.ReadAll(gzipReader)
		require.NoError(t, err)

		var receivedMetricsBatch []models.MetricsDTO
		err = json.Unmarshal(body, &receivedMetricsBatch)
		require.NoError(t, err)

		receivedMetrics = append(receivedMetrics, receivedMetricsBatch...)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)

	client := &http.Client{}

	err := sendMetrics(client, metrics, server.URL+"/update", key)
	require.NoError(t, err)

	// We should receive exactly one request with all metrics
	assert.Equal(t, len(metrics), len(receivedMetrics))

	receivedMetricsMap := make(map[string]models.MetricsDTO)
	for _, receivedMetric := range receivedMetrics {
		receivedMetricsMap[receivedMetric.ID] = receivedMetric
	}

	for _, metric := range metrics {
		_, exists := receivedMetricsMap[metric.Name]
		assert.True(t, exists, "Metric %s should be sent in a request", metric.Name)
	}
}

func TestSendMetricWithHash(t *testing.T) {
	var receivedMetrics []models.MetricsDTO
	key := "test-key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))

		// Check that HashSHA256 header is present and is a valid hex string
		hashHeader := r.Header.Get("HashSHA256")
		assert.NotEmpty(t, hashHeader)

		// Should be a valid hex string
		_, err := hex.DecodeString(hashHeader)
		assert.NoError(t, err, "HashSHA256 header should be a valid hex string")

		gzipReader, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		defer gzipReader.Close()

		body, err := io.ReadAll(gzipReader)
		require.NoError(t, err)

		var receivedMetricsBatch []models.MetricsDTO
		err = json.Unmarshal(body, &receivedMetricsBatch)
		require.NoError(t, err)

		receivedMetrics = append(receivedMetrics, receivedMetricsBatch...)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)

	client := &http.Client{}

	err := sendMetrics(client, metrics, server.URL+"/update", key)
	require.NoError(t, err)

	// We should receive exactly one request with all metrics
	assert.Equal(t, len(metrics), len(receivedMetrics))

	receivedMetricsMap := make(map[string]models.MetricsDTO)
	for _, receivedMetric := range receivedMetrics {
		receivedMetricsMap[receivedMetric.ID] = receivedMetric
	}

	for _, metric := range metrics {
		_, exists := receivedMetricsMap[metric.Name]
		assert.True(t, exists, "Metric %s should be sent in a request", metric.Name)
	}
}
