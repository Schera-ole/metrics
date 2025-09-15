package config

import (
	"flag"
	"os"
	"strconv"
)

type ServerConfig struct {
	Address         string
	StoreInterval   int
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
}

func NewServerConfig() (*ServerConfig, error) {
	config := &ServerConfig{
		Address:         "localhost:8080",
		StoreInterval:   300,
		FileStoragePath: "./cmd/server/logs",
		Restore:         false,
		DatabaseDSN:     "postgresql://schera:schera@localhost:5432/videos?sslmode=disable",
	}

	address := flag.String("a", config.Address, "address")
	storeInterval := flag.Int("i", config.StoreInterval, "store in file interval")
	fileStoragePath := flag.String("f", config.FileStoragePath, "path to store file")
	restoreFlag := flag.Bool("r", config.Restore, "bool flag, describe restore metrics from file or not")
	databaseDSN := flag.String("d", config.DatabaseDSN, "database dsn")
	flag.Parse()

	envVars := map[string]*string{
		"ADDRESS":           address,
		"FILE_STORAGE_PATH": fileStoragePath,
		"DATABASE_DSN":      databaseDSN,
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

	config.Address = *address
	config.StoreInterval = *storeInterval
	config.FileStoragePath = *fileStoragePath
	config.Restore = *restoreFlag

	return config, nil
}
