package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"time"

	"github.com/Schera-ole/metrics/internal/agent"
	"github.com/Schera-ole/metrics/internal/config"
)

var (
	counter int64
)

func collectMetrics() []agent.Metric {
	var metrics []agent.Metric
	var MemStats runtime.MemStats
	runtime.ReadMemStats(&MemStats)
	msValue := reflect.ValueOf(MemStats)
	msType := msValue.Type()
	for _, metric := range agent.RuntimeMetrics {
		field, _ := msType.FieldByName(metric)
		value := msValue.FieldByName(metric)
		metrics = append(metrics, agent.Metric{Name: field.Name, Type: config.GaugeType, Value: value})
	}
	counter += 1
	metrics = append(metrics, agent.Metric{Name: "RandomValue", Type: config.GaugeType, Value: rand.Float64})
	metrics = append(metrics, agent.Metric{Name: "PollCount", Type: config.CounterType, Value: counter})
	return metrics
}

func sendMetrics(metrics []agent.Metric, url string) error {
	for _, metric := range metrics {
		client := &http.Client{}
		endpoint := fmt.Sprintf("%s/%s/%s/%v", url, metric.Type, metric.Name, metric.Value)
		request, err := http.NewRequest(http.MethodPost, endpoint, nil)
		if err != nil {
			panic(err)
		}
		request.Header.Set("Content-Type", "text/plain")
		response, err := client.Do(request)
		if err != nil {
			panic(err)
		}
		io.Copy(os.Stdout, response.Body)
		response.Body.Close()
	}
	return nil
}

func main() {
	reportInterval := flag.Int("r", 10, "The frequency of sending metrics to the server")
	pollInterval := flag.Int("p", 2, "The frequency of polling metrics from the package")
	address := flag.String("a", "localhost:8080", "Address for sending metrics")
	flag.Parse()
	url := "http://" + *address + "/update"
	var metrics []agent.Metric
	go func() {
		for {
			metrics = collectMetrics()
			time.Sleep(time.Duration(*pollInterval) * time.Second)
		}
	}()
	for {
		err := sendMetrics(metrics, url)
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Duration(*reportInterval) * time.Second)
	}
}
