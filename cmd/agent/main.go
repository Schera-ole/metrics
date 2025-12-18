package main

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Schera-ole/metrics/internal/agent"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type Counter struct {
	Value int64
}

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
	metrics = append(metrics, agent.Metric{Name: "RandomValue", Type: models.Gauge, Value: rand.Float64()})
	metrics = append(metrics, agent.Metric{Name: "PollCount", Type: models.Counter, Value: counter.Value})

	return metrics
}

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

func countHash(compressedBody []byte, key string) []byte {
	keyBytes := []byte(key)
	h := hmac.New(sha256.New, keyBytes)
	h.Write(compressedBody)
	return h.Sum(nil)
}

func countHashString(compressedBody []byte, key string) string {
	hash := countHash(compressedBody, key)
	return fmt.Sprintf("%x", hash)
}

func sendMetrics(client *http.Client, metrics []agent.Metric, url string, key string) error {
	// Prepare the data to send
	var sendingData []models.MetricsDTO
	var hash string
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
		return fmt.Errorf("error creating json")
	}
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	if _, err := gzipWriter.Write(jsonData); err != nil {
		return fmt.Errorf("error compressing data: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("error closing gzip writer: %w", err)
	}
	if key != "" {
		hash = countHashString(compressedData.Bytes(), key)
	}

	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			if attempt <= len(delays) {
				delay := delays[attempt-1]
				fmt.Printf("Retry attempt %d after %v delay\n", attempt, delay)
				time.Sleep(delay)
			}
		}

		request, err := http.NewRequest(http.MethodPost, url, &compressedData)
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
				fmt.Printf("Retryable error occurred: %v\n", err)
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
			fmt.Printf("Response: %s\n", string(body))
			return nil
		} else {
			lastErr = fmt.Errorf("server returned error status %d: %s", response.StatusCode, string(body))
			// Think, that for 5xx errors, we should retry request
			if response.StatusCode >= 500 && response.StatusCode < 600 {
				fmt.Printf("Server error (5xx), will retry: %v\n", lastErr)
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

func collectGopsutilMetrics() []agent.Metric {
	var metrics []agent.Metric
	// Get memory metrics
	memory, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("error getting memory stats %v", err)
		return nil
	}
	metrics = append(metrics, agent.Metric{Name: "TotalMemory", Type: models.Gauge, Value: memory.Total})
	metrics = append(metrics, agent.Metric{Name: "FreeMemory", Type: models.Gauge, Value: memory.Free})

	// Get CPU metrics
	cpuPercents, err := cpu.Percent(time.Second, true)
	if err != nil {
		log.Printf("error getting cpu info %v", err)
		return nil
	}
	for i, percent := range cpuPercents {
		metrics = append(metrics, agent.Metric{Name: fmt.Sprintf("CPUutilization%d", i), Type: models.Gauge, Value: percent})
	}
	return metrics
}

func worker(client *http.Client, url string, key string, jobs <-chan []agent.Metric) {
	for job := range jobs {
		err := sendMetrics(client, job, url, key)
		if err != nil {
			log.Printf("Error sending metrics: %v", err)
		}
	}
}

func main() {
	agentConfig, err := agent.NewAgentConfig()
	if err != nil {
		log.Fatal("Failed to parse configuration: ", err)
	}

	client := &http.Client{}

	url := "http://" + agentConfig.Address + "/updates"
	counter := &Counter{Value: 0}
	jobs := make(chan []agent.Metric, 20)

	for w := 1; w <= agentConfig.RateLimit; w++ {
		go worker(client, url, agentConfig.Key, jobs)
	}
	go func() {
		for {
			jobs <- collectMetrics(counter)
			time.Sleep(time.Duration(agentConfig.PollInterval) * time.Second)
		}
	}()
	go func() {
		for {
			jobs <- collectGopsutilMetrics()
			time.Sleep(time.Duration(agentConfig.PollInterval) * time.Second)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// Block until signal received
	<-sigChan
	log.Println("Shutting down...")
	close(jobs)
}
