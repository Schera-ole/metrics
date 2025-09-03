package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Schera-ole/metrics/internal/handler"
	"github.com/Schera-ole/metrics/internal/repository"
	"go.uber.org/zap"
)

func main() {
	address := flag.String("a", "localhost:8080", "address")
	storeInterval := flag.Int("i", 300, "store in file interval")
	fileStoragePath := flag.String("f", "./cmd/server/logs", "path to store file")
	restoreFlag := flag.Bool("r", false, "bool flag, describe restore metrics from file or not")
	flag.Parse()

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize zap logger: ", err)
	}
	defer logger.Sync()
	logSugar := logger.Sugar()

	envVars := map[string]*string{
		"ADDRESS":           address,
		"FILE_STORAGE_PATH": fileStoragePath,
	}

	for envVar, flag := range envVars {
		if envValue := os.Getenv(envVar); envValue != "" {
			*flag = envValue
		}
	}

	if envStoreInterval := os.Getenv("STORE_INTERVAL"); envStoreInterval != "" {
		interval, err := strconv.Atoi(envStoreInterval)
		if err != nil {
			logSugar.Fatalf("Invalid value: %s", envStoreInterval)
		}
		*storeInterval = interval
	}

	if envRestoreFlag := os.Getenv("RESTORE"); envRestoreFlag != "" {
		restore, err := strconv.ParseBool(envRestoreFlag)
		if err != nil {
			logSugar.Fatalf("Invalid value: %s", envRestoreFlag)
		}
		*restoreFlag = restore
	}

	storage := repository.NewMemStorage()

	if *restoreFlag {
		storage.RestoreMetrics(*fileStoragePath, logSugar)
	}

	if *storeInterval == 0 {
		// This will be handled in the UpdateHandler
	} else {
		ticker := time.NewTicker(time.Duration(*storeInterval) * time.Second)
		defer ticker.Stop()

		go func() {
			for range ticker.C {
				if err := storage.SaveMetrics(*fileStoragePath); err != nil {
					logSugar.Errorf("Error saving metrics: %v", err)
				} else {
					logSugar.Info("Metrics saved to file")
				}
			}
		}()
	}

	logSugar.Infow(
		"Starting server",
		"address", address,
		"storeInterval", *storeInterval,
		"fileStoragePath", *fileStoragePath,
	)
	logSugar.Fatal(http.ListenAndServe(*address, handler.Router(storage, logSugar, *fileStoragePath, *storeInterval)))
}
