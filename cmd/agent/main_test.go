package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectMetrics(t *testing.T) {
	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)
	require.NotEmpty(t, metrics)
	for i, m := range metrics {
		t.Logf("Checking metric #%d: Name='%s', Type='%s', Value='%v'", i, m.Name, m.Type, m.Value)
		assert.NotEmpty(t, m.Name)
		assert.NotZero(t, m.Value)
	}
}

func TestSendMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
	}))
	defer server.Close()
	counter := &Counter{Value: 0}
	metrics := collectMetrics(counter)
	err := sendMetrics(metrics, server.URL+"/update")
	require.NoError(t, err)
}
