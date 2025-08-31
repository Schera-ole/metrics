package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	var receivedRequests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Capture the request body for verification
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		receivedRequests = append(receivedRequests, string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)
	err := sendMetrics(metrics, server.URL+"/update")
	require.NoError(t, err)

	// Verify that we received the expected number of requests
	assert.Equal(t, len(metrics), len(receivedRequests))

	// Verify that each metric was sent
	for _, metric := range metrics {
		// Check that the metric name appears in at least one request
		found := false
		for _, request := range receivedRequests {
			if strings.Contains(request, metric.Name) {
				found = true
				break
			}
		}
		assert.True(t, found, "Metric %s should be sent in a request", metric.Name)
	}
}
