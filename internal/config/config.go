package config

import "os"

const defaultServerAddress = "localhost:8080"

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
