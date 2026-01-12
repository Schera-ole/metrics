package middlewareinternal

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoggingMiddleware(t *testing.T) {
	// Create a test logger
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()
	logSugar := logger.Sugar()

	// Create a test handler that returns a simple response
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Wrap the handler with the logging middleware
	handler := LoggingMiddleware(logSugar)(nextHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Hello, World!", rec.Body.String())
}

func TestGzipMiddleware_NoGzipSupport(t *testing.T) {
	// Create a test handler that returns a simple response
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Wrap the handler with the gzip middleware
	handler := GzipMiddleware(nextHandler)

	// Create a test request without gzip support
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Hello, World!", rec.Body.String())
	// Check that no Content-Encoding header is set
	assert.Equal(t, "", rec.Header().Get("Content-Encoding"))
}

func TestGzipMiddleware_WithGzipSupport(t *testing.T) {
	// Create a test handler that returns a simple response
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Wrap the handler with the gzip middleware
	handler := GzipMiddleware(nextHandler)

	// Create a test request with gzip support
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	// Check that Content-Encoding header is set to gzip
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	// Decompress the response body
	reader, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer reader.Close()

	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, reader)
	require.NoError(t, err)

	// Check that the decompressed body is correct
	assert.Equal(t, "Hello, World!", decompressed.String())
}

func TestGzipMiddleware_LargeResponse(t *testing.T) {
	// Create a test handler that returns a large response
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Create a large response body
		response := strings.Repeat("Hello, World! ", 1000)
		w.Write([]byte(response))
	})

	// Wrap the handler with the gzip middleware
	handler := GzipMiddleware(nextHandler)

	// Create a test request with gzip support
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	// Check that Content-Encoding header is set to gzip
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	// Decompress the response body
	reader, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer reader.Close()

	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, reader)
	require.NoError(t, err)

	// Check that the decompressed body is correct
	expected := strings.Repeat("Hello, World! ", 1000)
	assert.Equal(t, expected, decompressed.String())
}

func TestLoggingResponseWriter_Write(t *testing.T) {
	// Create a test ResponseWriter
	rec := httptest.NewRecorder()

	// Create response data
	responseData := &responseData{
		status: 0,
		size:   0,
	}

	// Create logging response writer
	lw := loggingResponseWriter{
		ResponseWriter: rec,
		responseData:   responseData,
	}

	// Write some data
	data := []byte("Hello, World!")
	size, err := lw.Write(data)

	// Check results
	assert.NoError(t, err)
	assert.Equal(t, len(data), size)
	assert.Equal(t, len(data), responseData.size)
}

func TestLoggingResponseWriter_WriteHeader(t *testing.T) {
	// Create a test ResponseWriter
	rec := httptest.NewRecorder()

	// Create response data
	responseData := &responseData{
		status: 0,
		size:   0,
	}

	// Create logging response writer
	lw := loggingResponseWriter{
		ResponseWriter: rec,
		responseData:   responseData,
	}

	// Write header
	lw.WriteHeader(http.StatusNotFound)

	// Check results
	assert.Equal(t, http.StatusNotFound, responseData.status)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
