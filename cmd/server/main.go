package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Schera-ole/metrics/internal/config"
	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/migration"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
	_ "github.com/jackc/pgx/v5/stdlib"
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

	// Create repository
	var storage repository.Repository
	var metricsService *service.MetricsService
	if serverConfig.DatabaseDSN == "" {
		storage = repository.NewMemStorage()
		metricsService = service.NewMetricsService(storage)

		dir := filepath.Dir(serverConfig.FileStoragePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logSugar.Errorf("error creating directory: %w", err)
		}
		if serverConfig.Restore {
			restoreCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			metricsService.RestoreMetrics(restoreCtx, serverConfig.FileStoragePath, logSugar)
		}
		if serverConfig.StoreInterval == 0 {
			// This will be handled in the UpdateHandler
		} else {
			ticker := time.NewTicker(time.Duration(serverConfig.StoreInterval) * time.Second)
			defer ticker.Stop()

			go func() {
				for range ticker.C {
					backupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					if err := metricsService.SaveMetrics(backupCtx, serverConfig.FileStoragePath); err != nil {
						logSugar.Errorf("Error saving metrics: %v", err)
					} else {
						logSugar.Info("Metrics saved to file")
					}
					cancel()
				}
			}()
		}
	} else {
		migCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = migration.RunMigrations(migCtx, serverConfig.DatabaseDSN, logSugar)
		if err != nil {
			logSugar.Errorf("%v", err)
		}
		storage, err = repository.NewDBStorage(serverConfig.DatabaseDSN)
		if err != nil {
			logSugar.Fatalf("Error when open db connection: %v", err)
		}
		metricsService = service.NewMetricsService(storage)
		defer storage.Close()
	}

	logSugar.Infow(
		"Starting server",
		"address", serverConfig.Address,
		"storeInterval", serverConfig.StoreInterval,
		"fileStoragePath", serverConfig.FileStoragePath,
		"databaseDSN", serverConfig.DatabaseDSN,
	)

	logSugar.Fatal(
		http.ListenAndServe(
			serverConfig.Address,
			handler.Router(logSugar, serverConfig, metricsService),
		),
	)
}
