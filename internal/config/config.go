package config

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultServerAddress  = "localhost:8080"
	defaultPollInterval   = 2 * time.Second
	defaultReportInterval = 10 * time.Second
)

type ServerConfig struct {
	Address string
}

func NewServerConfig() ServerConfig {
	cfg := ServerConfig{Address: defaultServerAddress}

	if address := os.Getenv("ADDRESS"); address != "" {
		cfg.Address = address
	}

	return cfg
}

type AgentConfig struct {
	Address        string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

func NewAgentConfig() AgentConfig {
	cfg := AgentConfig{
		Address:        defaultServerAddress,
		PollInterval:   defaultPollInterval,
		ReportInterval: defaultReportInterval,
	}

	if address := os.Getenv("ADDRESS"); address != "" {
		cfg.Address = address
	}

	if d, ok := durationFromEnv("POLL_INTERVAL"); ok {
		cfg.PollInterval = d
	}

	if d, ok := durationFromEnv("REPORT_INTERVAL"); ok {
		cfg.ReportInterval = d
	}

	return cfg
}

func durationFromEnv(name string) (time.Duration, bool) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 0, false
	}

	return time.Duration(seconds) * time.Second, true
}
