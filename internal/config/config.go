// Package config provides configuration management for the metrics server.
//
// It handles parsing of command-line flags and environment variables.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
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

	// CryptoKey is the path to the file with private key
	CryptoKey string

	// TrustedSubnet is the trusted subnet for server
	TrustedSubnet string

	// GRPCServerAddress - grpc server will start on it
	GRPCServerAddress string
}

type ConfigFile struct {
	Address           string `json:"address"`
	Restore           bool   `json:"restore"`
	StoreInterval     int    `json:"store_interval"`
	FileStoragePath   string `json:"store_file"`
	DatabaseDSN       string `json:"database_dsn"`
	CryptoKey         string `json:"crypto_key"`
	TrustedSubnet     string `json:"trusted_subnet"`
	GRPCServerAddress string `json:"grpc_server_address"`
}

// NewServerConfig creates a new ServerConfig with default values and parses
// command-line flags and environment variables.
func NewServerConfig() (*ServerConfig, error) {
	var configFromFile ConfigFile
	config := &ServerConfig{
		Address:           "localhost:8080",
		StoreInterval:     300,
		FileStoragePath:   "./cmd/server/logs",
		Restore:           false,
		DatabaseDSN:       "",
		Key:               "",
		AuditFile:         "",
		AuditURL:          "",
		CryptoKey:         "./cmd/server/private.pem",
		TrustedSubnet:     "",
		GRPCServerAddress: "",
	}

	address := flag.String("a", config.Address, "address")
	storeInterval := flag.Int("i", config.StoreInterval, "store in file interval")
	fileStoragePath := flag.String("f", config.FileStoragePath, "path to store file")
	restoreFlag := flag.Bool("r", config.Restore, "bool flag, describe restore metrics from file or not")
	databaseDSN := flag.String("d", config.DatabaseDSN, "database dsn")
	key := flag.String("k", "", "Key for hash")
	auditFile := flag.String("audit-file", config.AuditFile, "file for audit log")
	auditURL := flag.String("audit-url", config.AuditURL, "url for audit log")
	cryptoKey := flag.String("crypto-key", config.CryptoKey, "path to private key")
	configFile := flag.String("c", "", "path to config file")
	trustedSubnet := flag.String("t", config.TrustedSubnet, "trusted subnet")
	grpcServerAddress := flag.String("g", "", "grpc server address")
	flag.Parse()
	if *configFile != "" {
		f, err := os.Open(*configFile)
		if err == nil {
			defer f.Close()
			decoder := json.NewDecoder(f)
			err = decoder.Decode(&configFromFile)
			if err != nil {
				fmt.Println("Error during decode JSON:", err)
			}
			config.Address = configFromFile.Address
			config.Restore = configFromFile.Restore
			config.StoreInterval = configFromFile.StoreInterval
			config.FileStoragePath = configFromFile.FileStoragePath
			config.DatabaseDSN = configFromFile.DatabaseDSN
			config.CryptoKey = configFromFile.CryptoKey
			config.TrustedSubnet = configFromFile.TrustedSubnet
			config.GRPCServerAddress = configFromFile.GRPCServerAddress
		} else {
			fmt.Println("Error during opening file:", err)
		}
	}
	envVars := map[string]*string{
		"ADDRESS":           address,
		"FILE_STORAGE_PATH": fileStoragePath,
		"DATABASE_DSN":      databaseDSN,
		"KEY":               key,
		"CRYPTO_KEY":        cryptoKey,
		"TRUSTED_SUBNET":    trustedSubnet,
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
	config.CryptoKey = *cryptoKey
	config.TrustedSubnet = *trustedSubnet
	config.GRPCServerAddress = *grpcServerAddress

	return config, nil
}
