package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/agent"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
)

func main() {
	cfg := config.NewAgentConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("metrics agent reports to %s every %s", cfg.Address, cfg.ReportInterval)

	agent.New(cfg).Run(ctx)
}
