package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
)

type (
	responseData struct {
		status int
		size   int
	}

	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

func loggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		logFn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			responseData := &responseData{
				status: 0,
				size:   0,
			}
			lw := loggingResponseWriter{
				ResponseWriter: w,
				responseData:   responseData,
			}
			uri := r.RequestURI
			method := r.Method

			next.ServeHTTP(&lw, r)
			duration := time.Since(start)

			logger.Infoln(
				"uri", uri,
				"method", method,
				"status", responseData.status,
				"duration", duration,
				"size", responseData.size,
			)

		}
		return http.HandlerFunc(logFn)
	}
}

func Router(storage *repository.MemStorage, logger *zap.SugaredLogger) chi.Router {
	router := chi.NewRouter()
	router.Use(loggingMiddleware(logger))
	// router.Use(middleware.StripSlashes)
	// router.Post("/update/{type}/{metric}/{value}", func(w http.ResponseWriter, r *http.Request) {
	// 	UpdateHandlerWithParams(w, r, storage)
	// })
	// router.Post("/update", func(w http.ResponseWriter, r *http.Request) {
	// 	UpdateHandler(w, r, storage)
	// })
	// router.Get("/value/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
	// 	GetHandler(w, r, storage)
	// })
	// router.Post("/value", func(w http.ResponseWriter, r *http.Request) {
	// 	GetValue(w, r, storage)
	// })
	router.Route("/update", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			UpdateHandler(w, r, storage)
		})
		r.Post("/{type}/{metric}/{value}", func(w http.ResponseWriter, r *http.Request) {
			UpdateHandlerWithParams(w, r, storage)
		})
	})

	router.Route("/value", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			GetValue(w, r, storage)
		})
		r.Get("/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
			GetHandler(w, r, storage)
		})
	})
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		GetListHandler(w, r, storage)
	})
	return router
}

func UpdateHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	var metrics models.Metrics
	err := json.NewDecoder(r.Body).Decode(&metrics)
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
