package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

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

func (m *MockedStorage) GetMetric(name string) (interface{}, error) {
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
	log_sugar := logger.Sugar()
	ts := httptest.NewServer(Router(storage, log_sugar))
	defer ts.Close()

	tests := []struct {
		name       string
		endpoint   string
		method     string
		statusCode int
	}{
		{
			name:       "positive gauge test",
			endpoint:   "/update/gauge/StackInuse/123",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
		},
		{
			name:       "positive counter test",
			endpoint:   "/update/counter/PollCounter/123",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
		},
		{
			name:       "not found test",
			endpoint:   "/update/gauge//123",
			method:     http.MethodPost,
			statusCode: http.StatusNotFound,
		},
		{
			name:       "bad request gauge test",
			endpoint:   "/update/gauge/StackInuse/qwerty",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "bad request counter test",
			endpoint:   "/update/counter/PollCounter/qwerty",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		fmt.Print(url + tt.endpoint)

		t.Run(tt.name, func(t *testing.T) {
			r := testRequest(t, ts, tt.method, tt.endpoint)
			defer r.Body.Close()
			assert.Equal(t, tt.statusCode, r.StatusCode)
		})
	}
}

func testRequest(t *testing.T, ts *httptest.Server, method,
	path string) *http.Response {
	req, err := http.NewRequest(method, ts.URL+path, nil)
	require.NoError(t, err)

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)

	return resp
}
