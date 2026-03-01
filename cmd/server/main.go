// Package main implements the metrics server application.
package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
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
	"google.golang.org/grpc"

	"github.com/Schera-ole/metrics/internal/audit"
	"github.com/Schera-ole/metrics/internal/config"
	grpc_server "github.com/Schera-ole/metrics/internal/grpc"
	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/migration"
	models "github.com/Schera-ole/metrics/internal/model"
	pb "github.com/Schera-ole/metrics/internal/proto"
	"github.com/Schera-ole/metrics/internal/repository"
	"github.com/Schera-ole/metrics/internal/service"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

// loadPrivateKey loads and parses an RSA private key from a file.
func loadPrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	privateKeyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file with private key: %w", err)
	}
	privateKeyBlock, _ := pem.Decode(privateKeyBytes)
	if privateKeyBlock == nil {
		return nil, fmt.Errorf("key is not found")
	}
	parsedPrivateKey, err := x509.ParsePKCS8PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	// Type assert to *rsa.PrivateKey
	privateKey, ok := parsedPrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("failed to type assert private key to *rsa.PrivateKey")
	}

	return privateKey, nil
}

// main initializes and starts the metrics server.
func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize zap logger: ", err)
	}
	defer logger.Sync()
	logSugar := logger.Sugar()

	// Print build information
	logSugar.Infof("Build version: %s", buildVersion)
	logSugar.Infof("Build date: %s", buildDate)
	logSugar.Infof("Build commit: %s", buildCommit)

	serverConfig, err := config.NewServerConfig()
	if err != nil {
		logSugar.Fatal("Failed to parse configuration: ", err)
	}

	privateKey, err := loadPrivateKey(serverConfig.CryptoKey)
	if err != nil {
		logSugar.Fatal("Failed to load private key: ", err)
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
		"auditFile", serverConfig.AuditFile,
		"auditURL", serverConfig.AuditURL,
		"trustedSubnet", serverConfig.TrustedSubnet,
		"GRPCServerAddress", serverConfig.GRPCServerAddress,
	)
	// create server
	router := handler.Router(logSugar, serverConfig, metricsService, auditLogger, privateKey)
	srv := &http.Server{
		Addr:    serverConfig.Address,
		Handler: router,
	}
	// create channels
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		logSugar.Infof("Starting server on %s", serverConfig.Address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
		close(serverErrChan)
	}()
	// Start grpc server
	var grpcServer *grpc.Server
	if serverConfig.GRPCServerAddress != "" {
		metricsServer := grpc_server.NewMetricsServer(metricsService)
		listen, err := net.Listen("tcp", serverConfig.GRPCServerAddress)
		if err != nil {
			logSugar.Fatalf("Failed to listen on gRPC address %s: %v", serverConfig.GRPCServerAddress, err)
		}
		grpcServer = grpc.NewServer(
			grpc.UnaryInterceptor(grpc_server.XRequestIPInterceptor(serverConfig.TrustedSubnet)),
		)
		pb.RegisterMetricsServer(grpcServer, metricsServer)
		go func() {
			if err := grpcServer.Serve(listen); err != nil {
				logSugar.Fatalf("gRPC server failed: %v", err)
			}
		}()
	}

	// Wait for interrupt signal or server error
	select {
	case err := <-serverErrChan:
		if err != nil {
			logSugar.Fatalf("Server failed to start: %v", err)
		}
	case <-sigChan:
		logSugar.Info("Shutting down server...")

		// Create shutdown context with timeout (30 seconds for graceful shutdown)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Channel to signal completion of shutdown
		done := make(chan struct{})

		go func() {
			// Shutdown the HTTP server gracefully
			if err := srv.Shutdown(shutdownCtx); err != nil {
				logSugar.Infof("HTTP server Shutdown: %v", err)
				if err := srv.Close(); err != nil {
					logSugar.Infof("HTTP server Close: %v", err)
				}
			}

			// Shutdown the gRPC server gracefully
			if grpcServer != nil {
				grpcServer.GracefulStop()
				logSugar.Info("gRPC server stopped gracefully")
			}

			close(done)
		}()

		// Wait and timeout (1 minute)
		select {
		case <-done:
			// Graceful shutdown completed
		case <-time.After(1 * time.Minute):
			logSugar.Info("Graceful shutdown timed out after 1 minute, forcing exit")
			// Force close
			if err := srv.Close(); err != nil {
				logSugar.Infof("HTTP server Close: %v", err)
			}
		}

		auditWg.Wait()
	}

	cancel()
}

// noopAuditLogger is a no-op implementation of AuditLogger for when auditing is disabled.
type noopAuditLogger struct{}

// Log implements the AuditLogger interface but does nothing.
func (n *noopAuditLogger) Log(metrics []string, ipAddress string) {
	// No-op implementation
}
