package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/handler"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func run() error {
	cfg, err := config.NewServerConfig()
	if err != nil {
		return err
	}

	storage := repository.NewMemStorage()
	metrics := service.NewMetricsService(storage)
	metricsHandler := handler.NewMetricsHandler(metrics)

	log.Printf("metrics server is listening on %s", cfg.Address)

	err = http.ListenAndServe(cfg.Address, metricsHandler.Router())
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
