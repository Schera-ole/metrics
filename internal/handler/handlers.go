package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Schera-ole/metrics/internal/config"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
)

func UpdateHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValue := chi.URLParam(r, "value")

	if metricName == "" {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	var Metric interface{}
	switch metricType {
	case config.GaugeType:
		floatVal, floatErr := strconv.ParseFloat(metricValue, 64)
		if floatErr != nil {
			http.Error(w, "Metric value should be a float", http.StatusBadRequest)
			return
		}
		Metric = floatVal
	case config.CounterType:
		intVal, intErr := strconv.ParseInt(metricValue, 10, 64)
		if intErr != nil {
			http.Error(w, "Metric value should be an integer", http.StatusBadRequest)
			return
		}
		Metric = intVal
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}
	err := storage.SetMetric(metricName, Metric, metricType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func GetHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	metricName := chi.URLParam(r, "name")
	metricValue, err := storage.GetMetric(metricName)
	if err != nil {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	fmt.Fprintf(w, "Metric '%s': %v\n", metricName, metricValue)
	w.WriteHeader(http.StatusOK)
}

func GetListHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	var result string
	metrics := storage.ListMetrics()

	for _, v := range metrics {
		result += fmt.Sprintf("%s: %s\n", v.Name, v.Value)
	}
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, result)
}
