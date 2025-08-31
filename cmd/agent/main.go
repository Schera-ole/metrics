package main

import (
	"bytes"
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
	"github.com/Schera-ole/metrics/internal/config"
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
		metrics = append(metrics, agent.Metric{Name: field.Name, Type: config.GaugeType, Value: value.Interface()})
	}
	counter.Value += 1
	metrics = append(metrics, agent.Metric{Name: "RandomValue", Type: config.GaugeType, Value: rand.Float64()})
	metrics = append(metrics, agent.Metric{Name: "PollCount", Type: config.CounterType, Value: counter.Value})

	return metrics
}

func sendMetrics(metrics []agent.Metric, url string) error {
	for _, metric := range metrics {
		client := &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
				MaxIdleConns:      0,
				IdleConnTimeout:   0,
			},
		}
		reqMetrics := models.Metrics{
			ID:    metric.Name,
			MType: metric.Type,
		}
		switch reqMetrics.MType {
		case config.GaugeType:
			if val, ok := metric.Value.(uint64); ok {
				floatVal := float64(val)
				reqMetrics.Value = &floatVal
			} else if val, ok := metric.Value.(float64); ok {
				reqMetrics.Value = &val
			} else if val, ok := metric.Value.(uint32); ok {
				floatVal := float64(val)
				reqMetrics.Value = &floatVal
			}
		case config.CounterType:
			if val, ok := metric.Value.(int64); ok {
				reqMetrics.Delta = &val
			}
		}
		jsonData, err := json.Marshal(reqMetrics)
		if err != nil {
			return fmt.Errorf("error creating json")
		}
		fmt.Printf("Sending JSON: %s\n", string(jsonData)) // Debug line
		request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("error creating request for %s", url)
		}
		request.Close = true
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("Error sending request for %s: %v\n", url, err)
			fmt.Printf("Request details - Method: %s, URL: %s, Content-Length: %d\n",
				request.Method, request.URL, request.ContentLength)
			return fmt.Errorf("error sending request for %s, %s", url, err)
		}
		defer response.Body.Close()
		io.Copy(os.Stdout, response.Body)
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
			err := sendMetrics(metrics, url)
			if err != nil {
				log.Fatal(err)
			}
		default:
			// при пустом - ничего не делаем.
		}
		time.Sleep(time.Duration(*reportInterval) * time.Second)
	}
}
