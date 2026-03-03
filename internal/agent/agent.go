// Package agent provides core functionality for the metrics collection agent.
package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	mathRand "math/rand"
	"net"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	grpcMetrics "github.com/Schera-ole/metrics/internal/proto"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"go.uber.org/zap"

	models "github.com/Schera-ole/metrics/internal/model"
)

// Counter tracks the number of times metrics have been collected.
type Counter struct {
	// Value is the current count of metric collection cycles
	Value int64
}

// CollectMetrics gathers runtime metrics from the Go runtime.
//
// It collects memory statistics, garbage collector information, and a random value.
func CollectMetrics(counter *Counter) []Metric {
	var metrics []Metric
	var MemStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&MemStats)
	msValue := reflect.ValueOf(MemStats)
	msType := msValue.Type()
	for _, metric := range RuntimeMetrics {
		field, _ := msType.FieldByName(metric)
		value := msValue.FieldByName(metric)
		metrics = append(metrics, Metric{Name: field.Name, Type: models.Gauge, Value: value.Interface()})
	}
	counter.Value++
	metrics = append(metrics, Metric{Name: "RandomValue", Type: models.Gauge, Value: mathRand.Float64()})
	metrics = append(metrics, Metric{Name: "PollCount", Type: models.Counter, Value: counter.Value})

	return metrics
}

// IsRetryableError determines if an error should trigger a retry.
//
// It checks for network-related errors and certain PostgreSQL errors.
func IsRetryableError(err error) bool {
	// Check if the error is a PostgreSQL error
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == pgerrcode.UniqueViolation {
			return true
		}
		if pgerrcode.IsConnectionException(pgErr.Code) {
			return true
		}
	}

	// Check any network errors
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "connection reset by peer") {
		return true
	}

	return false
}

// countHash calculates the HMAC SHA256 hash of the compressed body.
func countHash(compressedBody []byte, key string) []byte {
	keyBytes := []byte(key)
	h := hmac.New(sha256.New, keyBytes)
	h.Write(compressedBody)
	return h.Sum(nil)
}

// countHashString calculates the HMAC SHA256 hash of the compressed body and returns it as a hex string.
func countHashString(compressedBody []byte, key string) string {
	hash := countHash(compressedBody, key)
	return fmt.Sprintf("%x", hash)
}

// PrepareMetricsPayload prepares the metrics data for sending.
func PrepareMetricsPayload(metrics []Metric, key string, publicKey *rsa.PublicKey) ([]byte, string, error) {
	// Prepare the data to send
	var sendingData []models.MetricsDTO
	for _, metric := range metrics {
		reqMetrics := models.MetricsDTO{
			ID:    metric.Name,
			MType: metric.Type,
		}
		switch reqMetrics.MType {
		case models.Gauge:
			if val, ok := metric.Value.(uint64); ok {
				floatVal := float64(val)
				reqMetrics.Value = &floatVal
			} else if val, ok := metric.Value.(float64); ok {
				reqMetrics.Value = &val
			} else if val, ok := metric.Value.(uint32); ok {
				floatVal := float64(val)
				reqMetrics.Value = &floatVal
			}
		case models.Counter:
			if val, ok := metric.Value.(int64); ok {
				reqMetrics.Delta = &val
			}
		}
		sendingData = append(sendingData, reqMetrics)
	}
	jsonData, err := json.Marshal(sendingData)
	if err != nil {
		return nil, "", fmt.Errorf("error creating json")
	}
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	if _, err := gzipWriter.Write(jsonData); err != nil {
		return nil, "", fmt.Errorf("error compressing data: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, "", fmt.Errorf("error closing gzip writer: %w", err)
	}
	compressedBytes := compressedData.Bytes()
	encryptedMessage, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, compressedBytes)
	if err != nil {
		return nil, "", fmt.Errorf("error encrypting message: %w", err)
	}
	var hash string
	if key != "" {
		hash = countHashString(encryptedMessage, key)
	}
	return encryptedMessage, hash, nil
}

// SendWithRetry sends a batch of metrics to the server with retry logic.
func SendWithRetry(
	ctx context.Context,
	client *http.Client,
	payload []byte,
	hash string,
	url string,
	key string,
	ipAddress string,
	logSugar *zap.SugaredLogger,
) error {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt < 4; attempt++ {
		// Check if context was cancelled before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			if attempt <= len(delays) {
				delay := delays[attempt-1]
				logSugar.Infof("Retry attempt %d after %v delay\n", attempt, delay)

				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		// Create a new reader for each attempt since it gets consumed
		payloadReader := bytes.NewReader(payload)
		request, err := http.NewRequest(http.MethodPost, url, payloadReader)
		if err != nil {
			lastErr = fmt.Errorf("error creating request for %s: %w", url, err)
			continue
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Encoding", "gzip")
		request.Header.Set("X-Real-IP", ipAddress)
		if key != "" {
			request.Header.Set("HashSHA256", hash)
		}

		response, err := client.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("error sending request for %s: %w", url, err)
			// Check if the error is retryable
			if IsRetryableError(err) {
				logSugar.Infof("Retryable error occurred: %v", err)
				continue
			} else {
				return lastErr
			}
		}

		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("error reading response body: %w", err)
			continue
		}

		// Check response status code and decide retry or not
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			logSugar.Infof("Response: %s", string(body))
			return nil
		} else {
			lastErr = fmt.Errorf("server returned error status %d: %s", response.StatusCode, string(body))
			// Think, that for 5xx errors, we should retry request
			if response.StatusCode >= 500 && response.StatusCode < 600 {
				logSugar.Infof("Server error (5xx), will retry: %v", lastErr)
				continue
			} else {
				// For other errors, don't retry
				return lastErr
			}
		}
	}

	// Failed
	return fmt.Errorf("failed to send metrics after 4 attempts: %w", lastErr)
}

// CollectGopsutilMetrics gathers system metrics using gopsutil.
func CollectGopsutilMetrics(logSugar *zap.SugaredLogger) []Metric {
	var metrics []Metric
	// Get memory metrics
	memory, err := mem.VirtualMemory()
	if err != nil {
		logSugar.Error("Error getting memory stats", zap.Error(err))
		return nil
	}
	metrics = append(metrics, Metric{Name: "TotalMemory", Type: models.Gauge, Value: memory.Total})
	metrics = append(metrics, Metric{Name: "FreeMemory", Type: models.Gauge, Value: memory.Free})

	// Get CPU metrics
	cpuPercents, err := cpu.Percent(time.Second, true)
	if err != nil {
		logSugar.Error("Error getting CPU info", zap.Error(err))
		return nil
	}
	for i, percent := range cpuPercents {
		metrics = append(metrics, Metric{Name: fmt.Sprintf("CPUutilization%d", i), Type: models.Gauge, Value: percent})
	}
	return metrics
}

// Worker processes metric batches from the http jobs channel.
func Worker(
	ctx context.Context,
	client *http.Client,
	url string,
	key string,
	jobs <-chan []Metric,
	publicKey *rsa.PublicKey,
	wg *sync.WaitGroup,
	ipAddress string,
	logSugar *zap.SugaredLogger,
) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return // Channel closed
			}
			// Prepare the metrics payload
			payload, hash, err := PrepareMetricsPayload(job, key, publicKey)
			if err != nil {
				logSugar.Error("Error preparing metrics payload", zap.Error(err))
				continue
			}

			// Send with retry logic
			err = SendWithRetry(ctx, client, payload, hash, url, key, ipAddress, logSugar)
			if err != nil {
				logSugar.Error("Error sending metrics", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

// GRPCWorker processes metric batches from the grpc jobs channel.
func GRPCWorker(
	ctx context.Context,
	client grpcMetrics.MetricsClient,
	url string,
	jobs <-chan []Metric,
	wg *sync.WaitGroup,
	logSugar *zap.SugaredLogger,
) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return // Channel closed
			}
			// Prepare grpc message
			grpcMetricsList := make([]*grpcMetrics.Metric, 0, len(job))
			for _, m := range job {
				grpcMetric := grpcMetrics.Metric_builder{
					Id:   m.Name,
					Type: getGRPCMType(m.Type),
				}
				switch m.Type {
				case models.Gauge:
					if val, ok := m.Value.(uint64); ok {
						grpcMetric.Value = float64(val)
					} else if val, ok := m.Value.(float64); ok {
						grpcMetric.Value = val
					} else if val, ok := m.Value.(uint32); ok {
						grpcMetric.Value = float64(val)
					}
				case models.Counter:
					if val, ok := m.Value.(int64); ok {
						grpcMetric.Delta = val
					}
				}
				grpcMetricsList = append(grpcMetricsList, grpcMetric.Build())
			}

			// Create request with the metrics using builder
			req := grpcMetrics.UpdateMetricsRequest_builder{
				Metrics: grpcMetricsList,
			}.Build()
			logSugar.Infof("Sending %d metrics to gRPC server", len(grpcMetricsList))
			// Send to gRPC server
			_, err := client.UpdateMetrics(ctx, req)
			if err != nil {
				logSugar.Errorf("Error sending metrics via gRPC: %v", err)
			} else {
				logSugar.Infof("Successfully sent %d metrics to gRPC server", len(grpcMetricsList))
			}
		case <-ctx.Done():
			return
		}
	}
}

// getGRPCMType converts internal metric type to gRPC MType enum.
func getGRPCMType(mType string) grpcMetrics.Metric_MType {
	if mType == models.Gauge {
		return grpcMetrics.Metric_GAUGE
	}
	return grpcMetrics.Metric_COUNTER
}

// LoadPublicKey loads an RSA public key from a PEM file.
func LoadPublicKey(pathToFile string) (*rsa.PublicKey, error) {
	publicKeyBytes, err := os.ReadFile(pathToFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file with public key: %w", err)
	}
	publicKeyBlock, _ := pem.Decode(publicKeyBytes)
	if publicKeyBlock == nil {
		return nil, fmt.Errorf("key is not found")
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Type assert publicKey to *rsa.PublicKey
	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not an RSA public key")
	}
	return rsaPublicKey, nil
}

// GetOutboundIP returns the local IP address that would be used to reach the target.
func GetOutboundIP(target string) (string, error) {
	conn, err := net.Dial("udp", target)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP
	ipStr := ip.String()
	return ipStr, nil
}
