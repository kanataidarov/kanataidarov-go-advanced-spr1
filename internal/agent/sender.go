package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mailru/easyjson"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

const requestTimeout = 5 * time.Second

type Sender struct {
	client         *http.Client
	baseURL        string
	reportInterval time.Duration
}

func NewSender(cfg config.SenderConfig) *Sender {
	return &Sender{
		client:         &http.Client{Timeout: requestTimeout},
		baseURL:        normalizeBaseURL(cfg.Address),
		reportInterval: cfg.ReportInterval,
	}
}

func (s *Sender) ReportInterval() time.Duration {
	return s.reportInterval
}

func (s *Sender) Send(ctx context.Context, metric models.Metrics) error {
	body, err := easyjson.Marshal(metric)
	if err != nil {
		return fmt.Errorf("cannot encode metric %s: %w", metric.ID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/update/", bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func normalizeBaseURL(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return strings.TrimSuffix(address, "/")
	}

	return "http://" + strings.TrimSuffix(address, "/")
}
