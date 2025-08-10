package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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

func (m *MockedStorage) ListMetrics() []string {
	// Просто заглушка
	return nil
}

func TestUpdateHandler(t *testing.T) {
	url := "http://localhost:8080"
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
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, url+tt.endpoint, nil)
			w := httptest.NewRecorder()

			mockStorage := &MockedStorage{}
			UpdateHandler(w, r, mockStorage)
			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}
