package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"database/sql"

	"github.com/Schera-ole/metrics/internal/config"
	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
	_ "github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func main() {
	serverConfig, err := config.NewServerConfig()
	if err != nil {
		log.Fatal("Failed to parse configuration: ", err)
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize zap logger: ", err)
	}
	defer logger.Sync()
	logSugar := logger.Sugar()

	storage := repository.NewMemStorage()
	metricsService := service.NewMetricsService(storage)

	dir := filepath.Dir(serverConfig.FileStoragePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logSugar.Errorf("error creating directory: %w", err)
	}

	if serverConfig.Restore {
		metricsService.RestoreMetrics(serverConfig.FileStoragePath, logSugar)
	}

	if serverConfig.StoreInterval == 0 {
		// This will be handled in the UpdateHandler
	} else {
		ticker := time.NewTicker(time.Duration(serverConfig.StoreInterval) * time.Second)
		defer ticker.Stop()

		go func() {
			for range ticker.C {
				if err := metricsService.SaveMetrics(serverConfig.FileStoragePath); err != nil {
					logSugar.Errorf("Error saving metrics: %v", err)
				} else {
					logSugar.Info("Metrics saved to file")
				}
			}
		}()
	}

	logSugar.Infow(
		"Starting server",
		"address", serverConfig.Address,
		"storeInterval", serverConfig.StoreInterval,
		"fileStoragePath", serverConfig.FileStoragePath,
	)
	dbConnect, err := sql.Open("pgx", serverConfig.DatabaseDSN)
	if err != nil {
		logSugar.Errorf("Error when open db connection: %v", err)
	}
	defer dbConnect.Close()
	logSugar.Fatal(
		http.ListenAndServe(
			serverConfig.Address,
			handler.Router(storage, logSugar, serverConfig, metricsService, dbConnect),
		),
	)
}
