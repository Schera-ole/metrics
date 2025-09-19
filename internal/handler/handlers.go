package handler

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/config"
	middlewareinternal "github.com/Schera-ole/metrics/internal/middleware"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
)

func Router(
	storage repository.Repository,
	logger *zap.SugaredLogger,
	config *config.ServerConfig,
	metricService *service.MetricsService,
) chi.Router {
	router := chi.NewRouter()
	router.Use(middlewareinternal.LoggingMiddleware(logger))
	router.Use(middlewareinternal.GzipMiddleware)
	router.Use(middleware.StripSlashes)
	router.Use(middleware.Timeout(15 * time.Second))
	router.Post("/update/{type}/{metric}/{value}", func(w http.ResponseWriter, r *http.Request) {
		UpdateHandlerWithParams(w, r, storage, logger, config, metricService)
	})
	router.Post("/update", func(w http.ResponseWriter, r *http.Request) {
		UpdateHandler(w, r, storage, logger, config, metricService)
	})
	router.Post("/updates", func(w http.ResponseWriter, r *http.Request) {
		BatchUpdateHandler(w, r, storage, logger, config, metricService)
	})
	router.Get("/value/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
		GetHandler(w, r, storage)
	})
	router.Post("/value", func(w http.ResponseWriter, r *http.Request) {
		GetValue(w, r, storage, logger)
	})
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		PingDatabaseHandler(w, r, storage, logger)
	})
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		GetListHandler(w, r, storage)
	})
	return router
}

func BatchUpdateHandler(
	w http.ResponseWriter,
	r *http.Request,
	storage repository.Repository,
	logger *zap.SugaredLogger,
	config *config.ServerConfig,
	metricService *service.MetricsService,
) {
	var reader io.Reader = r.Body

	if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Failed to create gzip reader: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	var metrics []models.MetricsDTO
	err := json.NewDecoder(reader).Decode(&metrics)
	if err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}
	var preparedMetrics []models.Metric
	for _, d := range metrics {
		if d.Value != nil {
			preparedMetrics = append(preparedMetrics, models.Metric{
				Name:  d.ID,
				Type:  d.MType,
				Value: *d.Value,
			})
		}
		if d.Delta != nil {
			preparedMetrics = append(preparedMetrics, models.Metric{
				Name:  d.ID,
				Type:  d.MType,
				Value: *d.Delta,
			})
		}
	}
	err = storage.SetMetrics(r.Context(), preparedMetrics)
	if err != nil {
		logger.Info(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	if config.StoreInterval == 0 {
		// Only save to file if using MemStorage
		if _, isMemStorage := storage.(*repository.MemStorage); isMemStorage {
			if err := metricService.SaveMetrics(r.Context(), config.FileStoragePath); err != nil {
				logger.Infof("couldn't save to file %s", err)
			}
		}
	}

}
func PingDatabaseHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository, logger *zap.SugaredLogger) {
	err := storage.Ping(r.Context())
	if err != nil {
		logger.Errorf("%w", err)
		http.Error(w, "Failed to connect to database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func UpdateHandler(
	w http.ResponseWriter,
	r *http.Request,
	storage repository.Repository,
	logger *zap.SugaredLogger,
	config *config.ServerConfig,
	metricService *service.MetricsService,
) {
	var reader io.Reader = r.Body

	if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Failed to create gzip reader: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	var metrics models.MetricsDTO
	err := json.NewDecoder(reader).Decode(&metrics)
	if err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}
	switch metrics.MType {
	case models.Gauge:
		if metrics.Value == nil {
			http.Error(w, "Gauge metrics must have a value", http.StatusBadRequest)
			return
		}
		err = storage.SetMetric(r.Context(), metrics.ID, *metrics.Value, metrics.MType)
	case models.Counter:
		if metrics.Delta == nil {
			http.Error(w, "Counter metrics must have a delta", http.StatusBadRequest)
			return
		}
		err = storage.SetMetric(r.Context(), metrics.ID, *metrics.Delta, metrics.MType)
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

	if config.StoreInterval == 0 {
		// Only save to file if using MemStorage
		if _, isMemStorage := storage.(*repository.MemStorage); isMemStorage {
			if err := metricService.SaveMetrics(r.Context(), config.FileStoragePath); err != nil {
				logger.Infof("couldn't save to file %s", err)
			}
		}
	}
}

func UpdateHandlerWithParams(
	w http.ResponseWriter,
	r *http.Request,
	storage repository.Repository,
	logger *zap.SugaredLogger,
	config *config.ServerConfig,
	metricService *service.MetricsService,
) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "metric")
	metricValue := chi.URLParam(r, "value")
	if metricName == "" {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	var Metric any
	switch metricType {
	case models.Gauge:
		floatVal, floatErr := strconv.ParseFloat(metricValue, 64)
		if floatErr != nil {
			http.Error(w, "Metric value should be a float", http.StatusBadRequest)
			return
		}
		Metric = floatVal
	case models.Counter:
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
	err := storage.SetMetric(r.Context(), metricName, Metric, metricType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

	if config.StoreInterval == 0 {
		// Only save to file if using MemStorage
		if _, isMemStorage := storage.(*repository.MemStorage); isMemStorage {
			if err := metricService.SaveMetrics(r.Context(), config.FileStoragePath); err != nil {
				logger.Infof("couldn't save to file %s", err)
			}
		}
	}
}

func GetValue(w http.ResponseWriter, r *http.Request, storage repository.Repository, logger *zap.SugaredLogger) {
	var metrics models.MetricsDTO
	var responseMetric models.MetricsDTO
	err := json.NewDecoder(r.Body).Decode(&metrics)
	if err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}
	logger.Infof("Try to getting metric, %s", metrics)
	responseMetric, err = storage.GetMetric(r.Context(), metrics)
	if err != nil {
		logger.Errorf("Error occured %w", err)
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	logger.Infof("Response metric, %s", responseMetric.Value)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responseMetric)
}

func GetHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	metricName := chi.URLParam(r, "name")
	metricValue, err := storage.GetMetricByName(r.Context(), metricName)
	if err != nil {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%v", metricValue)
}

func GetListHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	var result string
	metrics, _ := storage.ListMetrics(r.Context())

	for _, v := range metrics {
		result += fmt.Sprintf("%s: %s\n", v.Name, v.Value)
	}
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, result)
	w.WriteHeader(http.StatusOK)
}
