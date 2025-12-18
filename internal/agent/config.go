package agent

import (
	"flag"
	"log"
	"os"
	"strconv"
)

type AgentConfig struct {
	ReportInterval int
	PollInterval   int
	Address        string
	Key            string
	RateLimit      int
}

func NewAgentConfig() (*AgentConfig, error) {
	config := &AgentConfig{
		// ReportInterval: 10,
		PollInterval: 2,
		Address:      "localhost:8080",
		Key:          "",
		RateLimit:    5,
	}

	pollInterval := flag.Int("p", 2, "The frequency of polling metrics from the package")
	address := flag.String("a", "localhost:8080", "Address for sending metrics")
	key := flag.String("k", "", "Key for hash")
	rateLimit := flag.Int("l", 5, "Rate limit")
	flag.Parse()
	envIntVars := map[string]*int{
		"POLL_INTERVAL": pollInterval,
		"RATE_LIMIT":    rateLimit,
	}

	envStrVars := map[string]*string{
		"ADDRESS": address,
		"KEY":     key,
	}

	for envVar, flag := range envIntVars {
		if envValue := os.Getenv(envVar); envValue != "" {
			interval, err := strconv.Atoi(envValue)
			if err != nil {
				log.Fatalf("Invalid %s value: %s", envVar, envValue)
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

	return config, nil
}
