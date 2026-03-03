// Package main implements the metrics collection agent.
//
// It collects system and runtime metrics at regular intervals and sends them
// to the metrics server. The agent supports configurable polling intervals,
// rate limiting, and secure communication with the server.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Schera-ole/metrics/internal/agent"
	grpcMetrics "github.com/Schera-ole/metrics/internal/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

// main initializes and starts the metrics collection agent.
func main() {
	agentConfig, err := agent.NewAgentConfig()
	if err != nil {
		log.Fatal("Failed to parse configuration: ", err)
	}

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

	rsaPublicKey, err := agent.LoadPublicKey(agentConfig.CryptoKey)
	if err != nil {
		logger.Fatal("Failed to get public key", zap.Error(err))
	}

	client := &http.Client{}
	ipAddress, err := agent.GetOutboundIP("8.8.8.8:53")
	if err != nil {
		logger.Fatal("Failed to get ip address", zap.Error(err))
	}
	url := "http://" + agentConfig.Address + "/updates"

	counter := &agent.Counter{Value: 0}
	jobs := make(chan []agent.Metric, 20)

	// Create context with cancel for graceful shutdown using NotifyContext
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	// Create WaitGroup to wait for all workers to finish
	var wg sync.WaitGroup

	// GRPC worker
	if agentConfig.GRPCServerAddress != "" {
		grpcConn, err := grpc.NewClient(agentConfig.GRPCServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Fatal("Failed to get ip address", zap.Error(err))
		}
		defer grpcConn.Close()
		ctx = metadata.AppendToOutgoingContext(ctx, "x-real-ip", ipAddress)
		grpcClient := grpcMetrics.NewMetricsClient(grpcConn)
		for w := 1; w <= agentConfig.RateLimit; w++ {
			wg.Add(1)
			go agent.GRPCWorker(ctx, grpcClient, url, jobs, &wg, logSugar)
		}
	} else {
		// REST worker
		for w := 1; w <= agentConfig.RateLimit; w++ {
			wg.Add(1)
			go agent.Worker(ctx, client, url, agentConfig.Key, jobs, rsaPublicKey, &wg, ipAddress, logSugar)
		}
	}
	// Goroutine for collecting runtime metrics
	go func() {
		ticker := time.NewTicker(time.Duration(agentConfig.PollInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				jobs <- agent.CollectMetrics(counter)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Goroutine for collecting system metrics
	go func() {
		ticker := time.NewTicker(time.Duration(agentConfig.PollInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				jobs <- agent.CollectGopsutilMetrics(logSugar)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	logger.Info("Shutting down...")

	// Close jobs channel
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	logger.Info("Shutdown complete")
}
