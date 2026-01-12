package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
)

// testSetup creates common test dependencies
func testSetup(t *testing.T) (*service.MetricsService, *config.ServerConfig, *zap.SugaredLogger) {
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	t.Cleanup(func() { logger.Sync() })
	logSugar := logger.Sugar()
	metricService := service.NewMetricsService(storage)

	// Create a test configuration
	testConfig := &config.ServerConfig{
		Address:         "localhost:8080",
		StoreInterval:   0,
		FileStoragePath: "/home/schera/metrics/tmp/test_metrics.json",
		Restore:         false,
	}

	return metricService, testConfig, logSugar
}

// testRequest performs an HTTP request to the test server
func testRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, ts.URL+path, body)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)

	return resp
}

type MockedStorage struct {
	SetMetricCalled bool
	Err             error
}

func (m *MockedStorage) SetMetric(ctx context.Context, name string, val interface{}, typ string) error {
	m.SetMetricCalled = true
	return m.Err
}

func (m *MockedStorage) SetMetrics(ctx context.Context, metrics []models.Metric) error {
	// Stub implementation
	return nil
}

func (m *MockedStorage) GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error) {
	// Stub implementation
	return models.MetricsDTO{}, nil
}

func (m *MockedStorage) GetMetricByName(ctx context.Context, name string) (interface{}, error) {
	// Stub implementation
	return nil, nil
}

func (m *MockedStorage) DeleteMetric(ctx context.Context, name string) error {
	// Stub implementation
	return nil
}

func (m *MockedStorage) ListMetrics(ctx context.Context) ([]models.Metric, error) {
	// Stub implementation
	return nil, nil
}

func (m *MockedStorage) Ping(ctx context.Context) error {
	// Stub implementation
	return nil
}

func (m *MockedStorage) Close() error {
	// Stub implementation
	return nil
}

// mockAuditLogger is a mock implementation of the AuditLogger interface for testing.
type mockAuditLogger struct {
	logCalls []auditLogCall
}

// auditLogCall represents a call to the Log method.
type auditLogCall struct {
	metrics   []string
	ipAddress string
}

// Log implements the AuditLogger interface.
func (m *mockAuditLogger) Log(metrics []string, ipAddress string) {
	m.logCalls = append(m.logCalls, auditLogCall{
		metrics:   metrics,
		ipAddress: ipAddress,
	})
}

func TestUpdateHandler(t *testing.T) {
	metricService, testConfig, logSugar := testSetup(t)
	mockAudit := &mockAuditLogger{}
	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, mockAudit))
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
		{
			name:       "invalid metric type test",
			endpoint:   "/update",
			body:       `{"id":"InvalidMetric","type":"invalid","value":123.0}`,
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
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

func TestGetHandler(t *testing.T) {
	metricService, testConfig, logSugar := testSetup(t)
	mockAudit := &mockAuditLogger{}

	// Set a gauge metric
	err := metricService.SetMetric(context.Background(), "TestGauge", 42.5, models.Gauge)
	require.NoError(t, err)

	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, mockAudit))
	defer ts.Close()

	r := testRequest(t, ts, http.MethodGet, "/value/gauge/TestGauge", nil)
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)
	bodyBytes, _ := io.ReadAll(r.Body)
	assert.Contains(t, string(bodyBytes), "42.5")
}

func TestGetValueHandler(t *testing.T) {
	metricService, testConfig, logSugar := testSetup(t)
	mockAudit := &mockAuditLogger{}

	// Set a counter metric
	err := metricService.SetMetric(context.Background(), "TestCounter", int64(10), models.Counter)
	require.NoError(t, err)

	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, mockAudit))
	defer ts.Close()

	requestBody := `{"id":"TestCounter","type":"counter"}`
	r := testRequest(t, ts, http.MethodPost, "/value", bytes.NewBufferString(requestBody))
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)
	var resp models.MetricsDTO
	err = json.NewDecoder(r.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, int64(10), *resp.Delta)

	// Test getting a non-existent metric
	requestBody2 := `{"id":"NonExistent","type":"counter"}`
	r2 := testRequest(t, ts, http.MethodPost, "/value", bytes.NewBufferString(requestBody2))
	defer r2.Body.Close()
	assert.Equal(t, http.StatusNotFound, r2.StatusCode)
}

func TestGetListHandler(t *testing.T) {
	metricService, testConfig, logSugar := testSetup(t)
	mockAudit := &mockAuditLogger{}

	_ = metricService.SetMetric(context.Background(), "M1", 1.0, models.Gauge)
	_ = metricService.SetMetric(context.Background(), "M2", int64(2), models.Counter)

	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, mockAudit))
	defer ts.Close()

	r := testRequest(t, ts, http.MethodGet, "/", nil)
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)
	bodyBytes, _ := io.ReadAll(r.Body)
	bodyStr := string(bodyBytes)
	assert.Contains(t, bodyStr, "M1")
	assert.Contains(t, bodyStr, "M2")
}

func TestPingHandler(t *testing.T) {
	metricService, testConfig, logSugar := testSetup(t)
	mockAudit := &mockAuditLogger{}
	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, mockAudit))
	defer ts.Close()

	r := testRequest(t, ts, http.MethodGet, "/ping", nil)
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)
}

func TestBatchUpdateHandler(t *testing.T) {
	metricService, testConfig, logSugar := testSetup(t)
	mockAudit := &mockAuditLogger{}
	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, mockAudit))
	defer ts.Close()

	// Prepare batch payload
	val := 3.14
	delta := int64(5)
	batch := []models.MetricsDTO{{ID: "B1", MType: models.Gauge, Value: &val}, {ID: "B2", MType: models.Counter, Delta: &delta}}
	data, _ := json.Marshal(batch)
	r := testRequest(t, ts, http.MethodPost, "/updates", bytes.NewReader(data))
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)

	// Verify stored metrics
	m1, err := metricService.GetMetricByName(context.Background(), "B1")
	require.NoError(t, err)
	assert.Equal(t, 3.14, m1)
	m2, err := metricService.GetMetricByName(context.Background(), "B2")
	require.NoError(t, err)
	assert.Equal(t, int64(5), m2)

	// Test batch update with invalid JSON
	r2 := testRequest(t, ts, http.MethodPost, "/updates", bytes.NewReader([]byte(`[{"invalid": json`)))
	defer r2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, r2.StatusCode)
}

func TestUpdateHandlerWithParams(t *testing.T) {
	metricService, testConfig, logSugar := testSetup(t)
	mockAudit := &mockAuditLogger{}
	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, mockAudit))
	defer ts.Close()

	tests := []struct {
		name          string
		endpoint      string
		method        string
		statusCode    int
		expectedValue interface{}
	}{
		{
			name:          "gauge via URL params",
			endpoint:      "/update/gauge/ParamGauge/7.5",
			method:        http.MethodPost,
			statusCode:    http.StatusOK,
			expectedValue: 7.5,
		},
		{
			name:          "counter via URL params",
			endpoint:      "/update/counter/ParamCounter/10",
			method:        http.MethodPost,
			statusCode:    http.StatusOK,
			expectedValue: int64(10),
		},
		{
			name:          "invalid gauge value",
			endpoint:      "/update/gauge/BadGauge/not_a_number",
			method:        http.MethodPost,
			statusCode:    http.StatusBadRequest,
			expectedValue: nil,
		},
		{
			name:          "invalid counter value",
			endpoint:      "/update/counter/BadCounter/not_a_number",
			method:        http.MethodPost,
			statusCode:    http.StatusBadRequest,
			expectedValue: nil,
		},
		{
			name:          "invalid metric type",
			endpoint:      "/update/invalid/InvalidType/123",
			method:        http.MethodPost,
			statusCode:    http.StatusBadRequest,
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRequest(t, ts, tt.method, tt.endpoint, nil)
			defer r.Body.Close()
			assert.Equal(t, tt.statusCode, r.StatusCode)

			// If the request was successful, verify the stored value
			if tt.statusCode == http.StatusOK {
				// Extract metric name from endpoint: /update/type/name/value
				parts := strings.Split(tt.endpoint, "/")
				if len(parts) >= 5 {
					metricName := parts[3] // Index 3 contains the metric name
					val, err := metricService.GetMetricByName(context.Background(), metricName)
					require.NoError(t, err)
					assert.Equal(t, tt.expectedValue, val)
				}
			}
		})
	}
}
