package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockedStorage struct {
	SetMetricCalled bool
	Err             error
}

func (m *MockedStorage) SetMetric(name string, val interface{}, typ string) error {
	m.SetMetricCalled = true
	return m.Err
}

func (m *MockedStorage) GetMetric(metrics models.Metrics) (interface{}, error) {
	// Просто заглушка
	return nil, nil
}

func (m *MockedStorage) DeleteMetric(name string) error {
	// Просто заглушка
	return nil
}

func (m *MockedStorage) ListMetrics() []struct {
	Name  string
	Value interface{}
} {
	// Просто заглушка
	return nil
}

func TestUpdateHandler(t *testing.T) {
	url := "http://localhost:8080"
	storage := repository.NewMemStorage()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	logSugar := logger.Sugar()
	ts := httptest.NewServer(Router(storage, logSugar))
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
