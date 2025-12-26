// Package config provides configuration management for the metrics server.
//
// It handles parsing of command-line flags and environment variables.
package config

import (
	"flag"
	"os"
	"strconv"
)

// ServerConfig holds the configuration settings for the metrics server.
type ServerConfig struct {
	// Address is the host:port combination for the server to listen on.
	Address string

	// StoreInterval is the interval in seconds between saves to file storage.
	// If 0, metrics are saved immediately after each update.
	StoreInterval int

	// FileStoragePath is the path to the file where metrics are stored when using file storage.
	FileStoragePath string

	// Restore indicates whether to restore metrics from file storage on startup.
	Restore bool

	// DatabaseDSN is the data source name for connecting to the PostgreSQL database.
	// If empty, file-based storage is used instead.
	DatabaseDSN string

	// Key is the secret key used for HMAC SHA256 hashing of requests and responses.
	Key string

	// AuditFile is the path to the file where audit logs are written.
	AuditFile string

	// AuditURL is the URL where audit logs are sent via HTTP POST.
	AuditURL string
}

// NewServerConfig creates a new ServerConfig with default values and parses
// command-line flags and environment variables.
func NewServerConfig() (*ServerConfig, error) {

	config := &ServerConfig{
		Address:         "localhost:8080",
		StoreInterval:   300,
		FileStoragePath: "./cmd/server/logs",
		Restore:         false,
		DatabaseDSN:     "",
		Key:             "",
		AuditFile:       "",
		AuditURL:        "",
	}

	address := flag.String("a", config.Address, "address")
	storeInterval := flag.Int("i", config.StoreInterval, "store in file interval")
	fileStoragePath := flag.String("f", config.FileStoragePath, "path to store file")
	restoreFlag := flag.Bool("r", config.Restore, "bool flag, describe restore metrics from file or not")
	databaseDSN := flag.String("d", config.DatabaseDSN, "database dsn")
	key := flag.String("k", "", "Key for hash")
	auditFile := flag.String("audit-file", config.AuditFile, "file for audit log")
	auditURL := flag.String("audit-url", config.AuditURL, "url for audit log")
	flag.Parse()

	envVars := map[string]*string{
		"ADDRESS":           address,
		"FILE_STORAGE_PATH": fileStoragePath,
		"DATABASE_DSN":      databaseDSN,
		"KEY":               key,
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
	if *auditFile != "" {
		config.AuditFile = *auditFile
	}
	if *auditURL != "" {
		config.AuditURL = *auditURL
	}
	config.Address = *address
	config.StoreInterval = *storeInterval
	config.FileStoragePath = *fileStoragePath
	config.Restore = *restoreFlag
	config.DatabaseDSN = *databaseDSN
	config.Key = *key

	return config, nil
}
