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
		client := &http.Client{}
		endpoint := fmt.Sprintf("%s/%s/%s/%v", url, metric.Type, metric.Name, metric.Value)
		request, err := http.NewRequest(http.MethodPost, endpoint, nil)
		if err != nil {
			return fmt.Errorf("error creating request for %s", endpoint)
		}
		request.Header.Set("Content-Type", "text/plain")
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("error sending request for %s", endpoint)
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
	counter := &Counter{Value: 0}
	// var metrics []agent.Metric
	// go func() {
	// 	for {
	// 		metrics = collectMetrics(counter)
	// 		time.Sleep(time.Duration(*pollInterval) * time.Second)
	// 	}
	// }()
	// for {
	// 	err := sendMetrics(metrics, url)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	time.Sleep(time.Duration(*reportInterval) * time.Second)
	// }
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
