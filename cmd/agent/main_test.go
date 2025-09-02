package main

import (
	"compress/gzip"
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

	// Check that PollCount metric exists and has correct type
	foundPollCount := false
	for _, m := range metrics {
		t.Logf("Checking metric: Name='%s', Type='%s', Value='%v'", m.Name, m.Type, m.Value)
		assert.NotEmpty(t, m.Name)

		if m.Name == "PollCount" {
			foundPollCount = true
			assert.Equal(t, "counter", m.Type)
			assert.Equal(t, int64(1), m.Value) // Should be 1 since we start with 0 and increment
		} else {
			assert.Equal(t, "gauge", m.Type)
		}
	}

	// Ensure PollCount metric was found
	assert.True(t, foundPollCount, "PollCount metric should be present")
}

func TestSendMetric(t *testing.T) {
	var receivedRequests []models.Metrics

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))

		// Decompress the gzipped body
		gzipReader, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		defer gzipReader.Close()

		body, err := io.ReadAll(gzipReader)
		require.NoError(t, err)

		var receivedMetric models.Metrics
		err = json.Unmarshal(body, &receivedMetric)
		require.NoError(t, err)

		receivedRequests = append(receivedRequests, receivedMetric)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)

	// Create a test HTTP client
	client := &http.Client{}

	err := sendMetrics(client, metrics, server.URL+"/update")
	require.NoError(t, err)

	assert.Equal(t, len(metrics), len(receivedRequests))

	// Create a map of received metrics for easier lookup
	receivedMetricsMap := make(map[string]models.Metrics)
	for _, receivedMetric := range receivedRequests {
		receivedMetricsMap[receivedMetric.ID] = receivedMetric
	}

	// Verify that each metric was sent correctly
	for _, metric := range metrics {
		_, exists := receivedMetricsMap[metric.Name]
		assert.True(t, exists, "Metric %s should be sent in a request", metric.Name)
	}
}
