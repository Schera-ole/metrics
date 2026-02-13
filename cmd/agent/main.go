// Package main implements the metrics collection agent.
//
// It collects system and runtime metrics at regular intervals and sends them
// to the metrics server. The agent supports configurable polling intervals,
// rate limiting, and secure communication with the server.
package main

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
	"log"
	mathRand "math/rand"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/agent"
	models "github.com/Schera-ole/metrics/internal/model"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

// Counter tracks the number of times metrics have been collected.
type Counter struct {
	// Value is the current count of metric collection cycles
	Value int64
}

// collectMetrics gathers runtime metrics from the Go runtime.
//
// It collects memory statistics, garbage collector information, and a random value.
func collectMetrics(counter *Counter) []agent.Metric {

	var metrics []agent.Metric
	var MemStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&MemStats)
	msValue := reflect.ValueOf(MemStats)
	msType := msValue.Type()
	for _, metric := range agent.RuntimeMetrics {
		field, _ := msType.FieldByName(metric)
		value := msValue.FieldByName(metric)
		metrics = append(metrics, agent.Metric{Name: field.Name, Type: models.Gauge, Value: value.Interface()})
	}
	counter.Value += 1
	metrics = append(metrics, agent.Metric{Name: "RandomValue", Type: models.Gauge, Value: mathRand.Float64()})
	metrics = append(metrics, agent.Metric{Name: "PollCount", Type: models.Counter, Value: counter.Value})

	return metrics
}

// isRetryableError determines if an error should trigger a retry.
//
// It checks for network-related errors and certain PostgreSQL errors.
func isRetryableError(err error) bool {

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

// prepareMetricsPayload prepares the metrics data for sending.
func prepareMetricsPayload(metrics []agent.Metric, key string, publicKey *rsa.PublicKey) ([]byte, string, error) {
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
	return compressedBytes, hash, nil
}

// sendWithRetry sends a batch of metrics to the server with retry logic.
func sendWithRetry(ctx context.Context, client *http.Client, payload []byte, hash string, url string, key string, logSugar *zap.SugaredLogger) error {
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
		if key != "" {
			request.Header.Set("HashSHA256", hash)
		}

		response, err := client.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("error sending request for %s: %w", url, err)
			// Check if the error is retryable
			if isRetryableError(err) {
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

// collectGopsutilMetrics gathers system metrics using gopsutil.
func collectGopsutilMetrics(logSugar *zap.SugaredLogger) []agent.Metric {

	var metrics []agent.Metric
	// Get memory metrics
	memory, err := mem.VirtualMemory()
	if err != nil {
		logSugar.Error("Error getting memory stats", zap.Error(err))
		return nil
	}
	metrics = append(metrics, agent.Metric{Name: "TotalMemory", Type: models.Gauge, Value: memory.Total})
	metrics = append(metrics, agent.Metric{Name: "FreeMemory", Type: models.Gauge, Value: memory.Free})

	// Get CPU metrics
	cpuPercents, err := cpu.Percent(time.Second, true)
	if err != nil {
		logSugar.Error("Error getting CPU info", zap.Error(err))
		return nil
	}
	for i, percent := range cpuPercents {
		metrics = append(metrics, agent.Metric{Name: fmt.Sprintf("CPUutilization%d", i), Type: models.Gauge, Value: percent})
	}
	return metrics
}

// worker processes metric batches from the jobs channel.
func worker(ctx context.Context, client *http.Client, url string, key string, jobs <-chan []agent.Metric, publicKey *rsa.PublicKey, wg *sync.WaitGroup, logSugar *zap.SugaredLogger) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return // Channel closed
			}
			// Prepare the metrics payload
			payload, hash, err := prepareMetricsPayload(job, key, publicKey)
			if err != nil {
				logSugar.Error("Error preparing metrics payload", zap.Error(err))
				continue
			}

			// Send with retry logic
			err = sendWithRetry(ctx, client, payload, hash, url, key, logSugar)
			if err != nil {
				logSugar.Error("Error sending metrics", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

func loadPublicKey(pathToFile string) (*rsa.PublicKey, error) {
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

// main initializes and starts the metrics collection agent.
func main() {
	agentConfig, err := agent.NewAgentConfig()
	if err != nil {
		log.Fatal("Failed to parse configuration: ", err)
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize zap logger: ", err)
	}
	defer logger.Sync()
	logSugar := logger.Sugar()

	// Print build information
	logSugar.Infof("Build version: %s", buildVersion)
	logSugar.Infof("Build date: %s", buildDate)
	logSugar.Infof("Build commit: %s", buildCommit)

	rsaPublicKey, err := loadPublicKey(agentConfig.CryptoKey)
	if err != nil {
		logger.Fatal("Failed to get public key", zap.Error(err))
	}

	client := &http.Client{}

	url := "http://" + agentConfig.Address + "/updates"
	counter := &Counter{Value: 0}
	jobs := make(chan []agent.Metric, 20)

	// Create context with cancel for graceful shutdown using NotifyContext
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	// Create WaitGroup to wait for all workers to finish
	var wg sync.WaitGroup

	for w := 1; w <= agentConfig.RateLimit; w++ {
		wg.Add(1)
		go worker(ctx, client, url, agentConfig.Key, jobs, rsaPublicKey, &wg, logSugar)
	}

	// Goroutine for collecting runtime metrics
	go func() {
		ticker := time.NewTicker(time.Duration(agentConfig.PollInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				jobs <- collectMetrics(counter)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Goroutine for collecting system metrics
	go func() {
		ticker := time.NewTicker(time.Duration(agentConfig.PollInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				jobs <- collectGopsutilMetrics(logSugar)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	logger.Info("Shutting down...")

	// Close jobs channel
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	logger.Info("Shutdown complete")
}
