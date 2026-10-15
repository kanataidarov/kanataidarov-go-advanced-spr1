package agent

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
)

type Agent struct {
	collector *Collector
	sender    *Sender
	log       *zap.Logger
}

func New(cfg config.AgentConfig, log *zap.Logger) *Agent {
	return &Agent{
		collector: NewCollector(cfg.Collector),
		sender:    NewSender(cfg.Sender, log),
		log:       log,
	}
}

func (a *Agent) Run(ctx context.Context) {
	pollTicker := time.NewTicker(a.collector.PollInterval())
	defer pollTicker.Stop()

	reportTicker := time.NewTicker(a.sender.ReportInterval())
	defer reportTicker.Stop()

	a.collector.Poll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			a.collector.Poll()
		case <-reportTicker.C:
			a.Report(ctx)
		}
	}
}

func (a *Agent) Report(ctx context.Context) {
	metrics := a.collector.Snapshot()
	if len(metrics) == 0 {
		return
	}

	if err := a.sender.SendBatch(ctx, metrics); err != nil {
		a.log.Error("cannot report metrics", zap.Int("count", len(metrics)), zap.Error(err))

		return
	}

	for _, metric := range metrics {
		if metric.ID == pollCountMetric && metric.Delta != nil && *metric.Delta > 0 {
			a.collector.ResetPollCount(*metric.Delta)
		}
	}
}
