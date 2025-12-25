package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/Schera-ole/metrics/internal/audit"
	"github.com/Schera-ole/metrics/internal/config"
	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/migration"
	models "github.com/Schera-ole/metrics/internal/model"
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
	// Create event channel
	var eventChan = make(chan models.AuditEvent, 100)
	if serverConfig.AuditFile != "" || serverConfig.AuditURL != "" {
		var subs []chan<- models.AuditEvent
		if serverConfig.AuditFile != "" {
			fileChan := make(chan models.AuditEvent, 50)
			subs = append(subs, fileChan)
			go audit.FileSubscriber(fileChan, *serverConfig)
		}
		if serverConfig.AuditURL != "" {
			urlChan := make(chan models.AuditEvent, 50)
			subs = append(subs, urlChan)
			go audit.URLSubscriber(urlChan, *serverConfig)
		}
		go audit.Broadcaster(eventChan, subs...)
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
			handler.Router(logSugar, serverConfig, metricsService, eventChan),
		),
	)
}
