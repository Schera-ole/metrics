package handler

import (
	"bytes"
	"context"
	"encoding/json"
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

	ts := httptest.NewServer(Router(logSugar, testConfig, metricService, nil))
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
func TestGetHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	metricService := service.NewMetricsService(storage)

	// Set a gauge metric
	err := metricService.SetMetric(context.Background(), "TestGauge", 42.5, models.Gauge)
	require.NoError(t, err)

	testConfig := &config.ServerConfig{Address: "localhost:8080"}
	ts := httptest.NewServer(Router(logger.Sugar(), testConfig, metricService, nil))
	defer ts.Close()

	r := testRequest(t, ts, http.MethodGet, "/value/gauge/TestGauge", nil)
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)
	bodyBytes, _ := io.ReadAll(r.Body)
	assert.Contains(t, string(bodyBytes), "42.5")
}

func TestGetValueHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	metricService := service.NewMetricsService(storage)

	// Set a counter metric
	err := metricService.SetMetric(context.Background(), "TestCounter", int64(10), models.Counter)
	require.NoError(t, err)

	testConfig := &config.ServerConfig{Address: "localhost:8080"}
	ts := httptest.NewServer(Router(logger.Sugar(), testConfig, metricService, nil))
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
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	metricService := service.NewMetricsService(storage)

	_ = metricService.SetMetric(context.Background(), "M1", 1.0, models.Gauge)
	_ = metricService.SetMetric(context.Background(), "M2", int64(2), models.Counter)

	testConfig := &config.ServerConfig{Address: "localhost:8080"}
	ts := httptest.NewServer(Router(logger.Sugar(), testConfig, metricService, nil))
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
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	metricService := service.NewMetricsService(storage)
	testConfig := &config.ServerConfig{Address: "localhost:8080"}
	ts := httptest.NewServer(Router(logger.Sugar(), testConfig, metricService, nil))
	defer ts.Close()

	r := testRequest(t, ts, http.MethodGet, "/ping", nil)
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)
}

func TestBatchUpdateHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	metricService := service.NewMetricsService(storage)
	testConfig := &config.ServerConfig{Address: "localhost:8080"}
	ts := httptest.NewServer(Router(logger.Sugar(), testConfig, metricService, nil))
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
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	metricService := service.NewMetricsService(storage)
	testConfig := &config.ServerConfig{Address: "localhost:8080"}
	ts := httptest.NewServer(Router(logger.Sugar(), testConfig, metricService, nil))
	defer ts.Close()

	// gauge via URL params
	r := testRequest(t, ts, http.MethodPost, "/update/gauge/ParamGauge/7.5", nil)
	defer r.Body.Close()
	assert.Equal(t, http.StatusOK, r.StatusCode)
	val, err := metricService.GetMetricByName(context.Background(), "ParamGauge")
	require.NoError(t, err)
	assert.Equal(t, 7.5, val)

	// counter via URL params
	r2 := testRequest(t, ts, http.MethodPost, "/update/counter/ParamCounter/10", nil)
	defer r2.Body.Close()
	assert.Equal(t, http.StatusOK, r2.StatusCode)
	val2, err := metricService.GetMetricByName(context.Background(), "ParamCounter")
	require.NoError(t, err)
	assert.Equal(t, int64(10), val2)

	// invalid gauge value
	r3 := testRequest(t, ts, http.MethodPost, "/update/gauge/BadGauge/not_a_number", nil)
	defer r3.Body.Close()
	assert.Equal(t, http.StatusBadRequest, r3.StatusCode)

	// invalid counter value
	r4 := testRequest(t, ts, http.MethodPost, "/update/counter/BadCounter/not_a_number", nil)
	defer r4.Body.Close()
	assert.Equal(t, http.StatusBadRequest, r4.StatusCode)

	// invalid metric type
	r5 := testRequest(t, ts, http.MethodPost, "/update/invalid/InvalidType/123", nil)
	defer r5.Body.Close()
	assert.Equal(t, http.StatusBadRequest, r5.StatusCode)
}
