package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockedStorage struct {
	SetMetricCalled bool
	Err             error
}

func (m *MockedStorage) SetMetric(ctx context.Context, name string, val interface{}, typ string) error {
	m.SetMetricCalled = true
	return m.Err
}

func (m *MockedStorage) SetMetrics(ctx context.Context, metrics []models.Metric) error {
	// Просто заглушка
	return nil
}

func (m *MockedStorage) GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error) {
	// Просто заглушка
	return models.MetricsDTO{}, nil
}

func (m *MockedStorage) GetMetricByName(ctx context.Context, name string) (interface{}, error) {
	// Просто заглушка
	return nil, nil
}

func (m *MockedStorage) DeleteMetric(ctx context.Context, name string) error {
	// Просто заглушка
	return nil
}

func (m *MockedStorage) ListMetrics(ctx context.Context) ([]models.Metric, error) {
	// Просто заглушка
	return nil, nil
}

func (m *MockedStorage) Ping(ctx context.Context) error {
	// Просто заглушка
	return nil
}

func (m *MockedStorage) Close() error {
	// Просто заглушка
	return nil
}

func TestUpdateHandler(t *testing.T) {
	url := "http://localhost:8080"
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	logSugar := logger.Sugar()
	metricService := service.NewMetricsService(storage)

	// Create a test configuration
	testConfig := &config.ServerConfig{
		Address:         "localhost:8080",
		StoreInterval:   0,
		FileStoragePath: "./tmp/test_metrics.json",
		Restore:         false,
	}

	ts := httptest.NewServer(Router(logSugar, testConfig, metricService))
	defer ts.Close()

	tests := []struct {
		name       string
		endpoint   string
		body       string
		method     string
		statusCode int
	}{
		{
			name:       "positive gauge test",
			endpoint:   "/update",
			body:       `{"id":"StackInuse","type":"gauge","value":123.0}`,
			method:     http.MethodPost,
			statusCode: http.StatusOK,
		},
		{
			name:       "positive counter test",
			endpoint:   "/update",
			body:       `{"id":"PollCounter","type":"counter","delta":123}`,
			method:     http.MethodPost,
			statusCode: http.StatusOK,
		},
		{
			name:       "bad request gauge test",
			endpoint:   "/update",
			body:       `{"id":"StackInuse","type":"gauge"}`, // Missing value
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "bad request counter test",
			endpoint:   "/update",
			body:       `{"id":"PollCounter","type":"counter"}`, // Missing delta
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "invalid json test",
			endpoint:   "/update",
			body:       `{"invalid": json`,
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		fmt.Print(url + tt.endpoint)

		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = bytes.NewBufferString(tt.body)
			}
			r := testRequest(t, ts, tt.method, tt.endpoint, body)
			defer r.Body.Close()
			assert.Equal(t, tt.statusCode, r.StatusCode)
		})
	}
}

func testRequest(t *testing.T, ts *httptest.Server, method,
	path string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, ts.URL+path, body)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)

	return resp
}
