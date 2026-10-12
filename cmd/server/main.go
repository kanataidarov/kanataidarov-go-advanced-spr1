package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"go.uber.org/zap"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/handler"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/logger"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

func main() {
	zlog, err := logger.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot initialize logger: %v\n", err)
		os.Exit(1)
	}

	if err := run(zlog); err != nil {
		zlog.Fatal("server stopped", zap.Error(err))
	}

	_ = zlog.Sync()
}

func run(zlog *zap.Logger) error {
	cfg, err := config.NewServerConfig()
	if err != nil {
		return err
	}

	storage := repository.NewMemStorage()
	metrics := service.NewMetricsService(storage)
	metricsHandler := handler.NewMetricsHandler(metrics, zlog)

	router := logger.RequestLogger(zlog)(metricsHandler.Router())

	zlog.Info("metrics server is listening", zap.String("address", cfg.Address))

	err = http.ListenAndServe(cfg.Address, router)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
