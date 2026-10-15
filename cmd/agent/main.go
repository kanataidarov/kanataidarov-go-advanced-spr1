package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/agent"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/logger"
)

func main() {
	zlog, err := logger.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot initialize logger: %v\n", err)
		os.Exit(1)
	}

	if err := run(zlog); err != nil {
		zlog.Fatal("cannot start agent", zap.Error(err))
	}

	_ = zlog.Sync()
}

func run(zlog *zap.Logger) error {
	cfg, err := config.NewAgentConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	zlog.Info("metrics agent started",
		zap.String("address", cfg.Sender.Address),
		zap.Duration("report_interval", cfg.Sender.ReportInterval),
		zap.Duration("poll_interval", cfg.Collector.PollInterval),
	)

	agent.New(cfg, zlog).Run(ctx)

	return nil
}
