package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Schera-ole/metrics/internal/config"
	"github.com/Schera-ole/metrics/internal/repository"
)

func UpdateHandler(w http.ResponseWriter, r *http.Request, storage repository.Repository) {
	path := r.URL.Path
	pathData := strings.Split(path, "/")

	if len(pathData) != 5 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	metricType := pathData[2]
	metricName := pathData[3]
	metricValue := pathData[4]

	if metricName == "" {
		http.Error(w, "Metric name not found ", http.StatusNotFound)
		return
	}
	var Metric interface{}

	if metricType == config.GaugeType {
		floatVal, floatErr := strconv.ParseFloat(metricValue, 64)
		if floatErr != nil {
			http.Error(w, "Metric value should be a float", http.StatusBadRequest)
			return
		}
		Metric = floatVal
	} else if metricType == config.CounterType {
		intVal, intErr := strconv.ParseInt(metricValue, 10, 64)
		if intErr != nil {
			http.Error(w, "Metric value should be an integer", http.StatusBadRequest)
			return
		}
		Metric = intVal
	} else {
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
