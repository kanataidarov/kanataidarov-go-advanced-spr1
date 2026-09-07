package agent

import (
	"context"
	"log"
	"time"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
)

type Agent struct {
	collector *Collector
	sender    *Sender
}

func New(cfg config.AgentConfig) *Agent {
	return &Agent{
		collector: NewCollector(cfg.Collector),
		sender:    NewSender(cfg.Sender),
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
	var (
		reportedPollCount int64
		failed            bool
	)

	for _, metric := range a.collector.Snapshot() {
		if err := a.sender.Send(ctx, metric); err != nil {
			log.Printf("cannot report metric %s: %v", metric.ID, err)

			failed = true

			continue
		}

		if metric.ID == pollCountMetric && metric.Delta != nil {
			reportedPollCount = *metric.Delta
		}
	}

	if !failed && reportedPollCount > 0 {
		a.collector.ResetPollCount(reportedPollCount)
	}
}
