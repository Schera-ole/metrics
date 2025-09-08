package config

import (
	"flag"
	"os"
	"strconv"
)

// ServerConfig holds the configuration for the metrics server
type ServerConfig struct {
	Address         string
	StoreInterval   int
	FileStoragePath string
	Restore         bool
}

// NewServerConfig creates a new ServerConfig with default values and parses flags/environment variables
func NewServerConfig() (*ServerConfig, error) {
	config := &ServerConfig{
		Address:         "localhost:8080",
		StoreInterval:   300,
		FileStoragePath: "./cmd/server/logs",
		Restore:         false,
	}

	// Parse command line flags
	address := flag.String("a", config.Address, "address")
	storeInterval := flag.Int("i", config.StoreInterval, "store in file interval")
	fileStoragePath := flag.String("f", config.FileStoragePath, "path to store file")
	restoreFlag := flag.Bool("r", config.Restore, "bool flag, describe restore metrics from file or not")
	flag.Parse()

	// Override with environment variables if set
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
			return nil, err
		}
		*storeInterval = interval
	}

	if envRestoreFlag := os.Getenv("RESTORE"); envRestoreFlag != "" {
		restore, err := strconv.ParseBool(envRestoreFlag)
		if err != nil {
			return nil, err
		}
		*restoreFlag = restore
	}

	// Update config with parsed values
	config.Address = *address
	config.StoreInterval = *storeInterval
	config.FileStoragePath = *fileStoragePath
	config.Restore = *restoreFlag

	return config, nil
}
