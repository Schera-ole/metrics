package handler

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/config"
	middlewareInternal "github.com/Schera-ole/metrics/internal/middleware"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
)

func Router(storage *repository.MemStorage, logger *zap.SugaredLogger) chi.Router {
	router := chi.NewRouter()
	router.Use(middlewareInternal.LoggingMiddleware(logger))
	router.Use(middleware.StripSlashes)
	router.Post("/update/{type}/{metric}/{value}", func(w http.ResponseWriter, r *http.Request) {
		UpdateHandlerWithParams(w, r, storage)
	})
	router.Post("/update", func(w http.ResponseWriter, r *http.Request) {
		UpdateHandler(w, r, storage)
	})
	router.Get("/value/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
		GetHandler(w, r, storage)
	})
	router.Post("/value", func(w http.ResponseWriter, r *http.Request) {
		GetValue(w, r, storage)
	})

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		GetListHandler(w, r, storage)
	})
	return router
}

func UpdateHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	var reader io.Reader = r.Body

	// Check if the request is gzip compressed
	if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Failed to create gzip reader: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	var metrics models.Metrics
	err := json.NewDecoder(reader).Decode(&metrics)
	if err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}
	switch metrics.MType {
	case config.GaugeType:
		if metrics.Value == nil {
			http.Error(w, "Gauge metrics must have a value", http.StatusBadRequest)
			return
		}
		err = storage.SetMetric(metrics.ID, *metrics.Value, metrics.MType)
	case config.CounterType:
		if metrics.Delta == nil {
			http.Error(w, "Counter metrics must have a delta", http.StatusBadRequest)
			return
		}
		err = storage.SetMetric(metrics.ID, *metrics.Delta, metrics.MType)
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func UpdateHandlerWithParams(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "metric")
	metricValue := chi.URLParam(r, "value")
	if metricName == "" {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	var Metric any
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

func GetValue(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	var metrics models.Metrics
	err := json.NewDecoder(r.Body).Decode(&metrics)
	if err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}
	responseMetric, err := storage.GetMetricWithModels(metrics)
	if err != nil {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responseMetric)
}

func GetHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	metricName := chi.URLParam(r, "name")
	metricValue, err := storage.GetMetric(metricName)
	if err != nil {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	fmt.Fprintf(w, "%v", metricValue)
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
