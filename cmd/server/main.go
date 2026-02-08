// Package main implements the metrics server application.
package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/Schera-ole/metrics/internal/audit"
	"github.com/Schera-ole/metrics/internal/config"
	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/migration"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

// main initializes and starts the metrics server.
func main() {
	// Print build information
	log.Printf("Build version: %s\n", buildVersion)
	log.Printf("Build date: %s\n", buildDate)
	log.Printf("Build commit: %s\n", buildCommit)

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
	privateKeyBytes, err := os.ReadFile(serverConfig.CryptoKey)
	if err != nil {
		log.Fatal("Failed to read file with private key ", err)
	}
	privateKeyBlock, _ := pem.Decode(privateKeyBytes)
	if privateKeyBlock == nil {
		log.Fatal("Key is not found")
	}
	parsedPrivateKey, err := x509.ParsePKCS8PrivateKey(privateKeyBlock.Bytes)

	if err != nil {
		log.Fatal(err)
	}

	// Type assert to *rsa.PrivateKey
	privateKey, ok := parsedPrivateKey.(*rsa.PrivateKey)
	if !ok {
		log.Fatal("Failed to type assert private key to *rsa.PrivateKey")
	}

	// Create repository
	var storage repository.Repository
	var metricsService *service.MetricsService

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if serverConfig.DatabaseDSN == "" {
		storage = repository.NewMemStorage()
		metricsService = service.NewMetricsService(storage)

		dir := filepath.Dir(serverConfig.FileStoragePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logSugar.Errorf("error creating directory: %w", err)
		}
		if serverConfig.Restore {
			restoreCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			metricsService.RestoreMetrics(restoreCtx, serverConfig.FileStoragePath, logSugar)
		}
		if serverConfig.StoreInterval == 0 {
			// This will be handled in the UpdateHandler
		} else {
			ticker := time.NewTicker(time.Duration(serverConfig.StoreInterval) * time.Second)
			defer ticker.Stop()

			go func() {
				for {
					select {
					case <-ticker.C:
						backupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
						if err := metricsService.SaveMetrics(backupCtx, serverConfig.FileStoragePath); err != nil {
							logSugar.Errorf("Error saving metrics: %v", err)
						} else {
							logSugar.Info("Metrics saved to file")
						}
						cancel()
					case <-ctx.Done():
						logSugar.Info("Backup goroutine shutting down")
						return
					}
				}
			}()
		}
	} else {
		migCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
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
		go func() {
			<-ctx.Done()
			if storage != nil {
				storage.Close()
			}
		}()
	}
	var auditWg sync.WaitGroup
	// Create event channel
	var eventChan = make(chan models.AuditEvent, 100)
	if serverConfig.AuditFile != "" || serverConfig.AuditURL != "" {
		var subs []chan<- models.AuditEvent
		if serverConfig.AuditFile != "" {
			fileChan := make(chan models.AuditEvent, 50)
			subs = append(subs, fileChan)
			auditWg.Add(1)
			go func() {
				defer auditWg.Done()
				audit.FileSubscriber(ctx, fileChan, *serverConfig)
			}()
		}
		if serverConfig.AuditURL != "" {
			urlChan := make(chan models.AuditEvent, 50)
			subs = append(subs, urlChan)
			auditWg.Add(1)
			go func() {
				defer auditWg.Done()
				audit.URLSubscriber(ctx, urlChan, *serverConfig)
			}()
		}
		auditWg.Add(1)
		go func() {
			defer auditWg.Done()
			audit.Broadcaster(ctx, eventChan, subs...)
		}()
	}

	// Create audit logger
	var auditLogger audit.AuditLogger
	if serverConfig.AuditFile != "" || serverConfig.AuditURL != "" {
		auditLogger = audit.NewAuditLogger(eventChan)
	} else {
		// Create a no-op audit logger if auditing is disabled
		auditLogger = &noopAuditLogger{}
	}

	logSugar.Infow(
		"Starting server",
		"address", serverConfig.Address,
		"storeInterval", serverConfig.StoreInterval,
		"fileStoragePath", serverConfig.FileStoragePath,
		"databaseDSN", serverConfig.DatabaseDSN,
	)
	// create server
	router := handler.Router(logSugar, serverConfig, metricsService, auditLogger, privateKey)
	srv := &http.Server{
		Addr:    serverConfig.Address,
		Handler: router,
	}
	// create chanels
	idleConnsClosed := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigChan
		log.Print("Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}
		auditWg.Wait()
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}

	<-idleConnsClosed
	cancel()
}

// noopAuditLogger is a no-op implementation of AuditLogger for when auditing is disabled.
type noopAuditLogger struct{}

// Log implements the AuditLogger interface but does nothing.
func (n *noopAuditLogger) Log(metrics []string, ipAddress string) {
	// No-op implementation
}
