package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/Schera-ole/metrics/internal/agent"
	models "github.com/Schera-ole/metrics/internal/model"
)

type Counter struct {
	Value int64
}

func collectMetrics(counter *Counter) []agent.Metric {
	var metrics []agent.Metric
	var MemStats runtime.MemStats
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

func sendMetrics(client *http.Client, metrics []agent.Metric, url string) error {
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
		jsonData, err := json.Marshal(reqMetrics)
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

		request, err := http.NewRequest(http.MethodPost, url, &compressedData)
		if err != nil {
			return fmt.Errorf("error creating request for %s", url)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept-Encoding", "gzip")
		request.Header.Set("Content-Encoding", "gzip")

		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("error sending request for %s, %s", url, err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return fmt.Errorf("error reading response body: %s", err)
		}
		fmt.Printf("Response: %s\n", string(body))
	}
	return nil
}

func main() {
	reportInterval := flag.Int("r", 10, "The frequency of sending metrics to the server")
	pollInterval := flag.Int("p", 2, "The frequency of polling metrics from the package")
	address := flag.String("a", "localhost:8080", "Address for sending metrics")
	flag.Parse()
	envVars := map[string]*int{
		"REPORT_INTERVAL": reportInterval,
		"POLL_INTERVAL":   pollInterval,
	}

	for envVar, flag := range envVars {
		if envValue := os.Getenv(envVar); envValue != "" {
			interval, err := strconv.Atoi(envValue)
			if err != nil {
				log.Fatalf("Invalid %s value: %s", envVar, envValue)
			}
			*flag = interval
		}
	}

	if envAddress := os.Getenv("ADDRESS"); envAddress != "" {
		*address = envAddress
	}

	client := &http.Client{}

	url := "http://" + *address + "/update"
	counter := &Counter{Value: 0}
	metricsCh := make(chan []agent.Metric, 10)
	go func() {
		for {
			metricsCh <- collectMetrics(counter)
			time.Sleep(time.Duration(*pollInterval) * time.Second)
		}
	}()
	for {
		select {
		case metrics := <-metricsCh:
			err := sendMetrics(client, metrics, url)
			if err != nil {
				log.Printf("Error sending metrics: %v", err)
			}
		default:
			// при пустом - ничего не делаем.
		}
		time.Sleep(time.Duration(*reportInterval) * time.Second)
	}
}
