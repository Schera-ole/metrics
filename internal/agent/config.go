// Package agent provides configuration management for the metrics collection agent.
package agent

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
)

// AgentConfig holds the configuration settings for the metrics collection agent.
type AgentConfig struct {
	// ReportInterval is the interval in seconds between reports to the server.
	ReportInterval int

	// PollInterval is the interval in seconds between metric collections.
	PollInterval int

	// Address is the host:port combination of the metrics server to send data to.
	Address string

	// Key is the secret key used for HMAC SHA256 hashing of requests.
	Key string

	// RateLimit is the maximum number of concurrent requests to the server.
	RateLimit int

	// CryptoKey is the path to the file with public key
	CryptoKey string

	//GrpcServerAddress if present - agent will send updates to it
	GRPCServerAddress string
}

type ConfigFile struct {
	Address           string `json:"address"`
	RateLimit         int    `json:"rate_limit"`
	PollInterval      int    `json:"poll_interval"`
	CryptoKey         string `json:"crypto_key"`
	GRPCServerAddress string `json:"grpc_server_address"`
}

// NewAgentConfig creates a new AgentConfig with default values and parses
// command-line flags and environment variables.
func NewAgentConfig() (*AgentConfig, error) {
	var configFromFile ConfigFile
	config := &AgentConfig{
		// ReportInterval: 10,
		PollInterval:      2,
		Address:           "localhost:8080",
		Key:               "",
		RateLimit:         5,
		CryptoKey:         "./cmd/server/public.pem",
		GRPCServerAddress: "",
	}

	pollInterval := flag.Int("p", 2, "The frequency of polling metrics from the package")
	address := flag.String("a", "localhost:8080", "Address for sending metrics")
	key := flag.String("k", "", "Key for hash")
	rateLimit := flag.Int("l", 5, "Rate limit")
	cryptoKey := flag.String("crypto-key", config.CryptoKey, "path to public key")
	configFile := flag.String("c", "", "path to config file")
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
			config.PollInterval = configFromFile.PollInterval
			config.RateLimit = configFromFile.RateLimit
			config.CryptoKey = configFromFile.CryptoKey
			config.GRPCServerAddress = configFromFile.GRPCServerAddress
		} else {
			fmt.Println("Error during opening file:", err)
		}
	}
	envIntVars := map[string]*int{
		"POLL_INTERVAL": pollInterval,
		"RATE_LIMIT":    rateLimit,
	}

	envStrVars := map[string]*string{
		"ADDRESS":    address,
		"KEY":        key,
		"CRYPTO_KEY": cryptoKey,
	}

	for envVar, flag := range envIntVars {
		if envValue := os.Getenv(envVar); envValue != "" {
			interval, err := strconv.Atoi(envValue)
			if err != nil {
				return nil, fmt.Errorf("invalid %s value: %s", envVar, envValue)
			}
			*flag = interval
		}
	}

	for envVar, flag := range envStrVars {
		if envValue := os.Getenv(envVar); envValue != "" {
			*flag = envValue
		}
	}
	config.Address = *address
	config.PollInterval = *pollInterval
	config.RateLimit = *rateLimit
	config.Key = *key
	config.CryptoKey = *cryptoKey
	config.GRPCServerAddress = *grpcServerAddress

	return config, nil
}
