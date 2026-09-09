package agent

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	value, err := metricValue(metric)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/update/%s/%s/%s", s.baseURL, metric.MType, metric.ID, value)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func metricValue(metric models.Metrics) (string, error) {
	switch metric.MType {
	case models.Gauge:
		if metric.Value == nil {
			return "", fmt.Errorf("gauge %s has no value", metric.ID)
		}

		return strconv.FormatFloat(*metric.Value, 'f', -1, 64), nil
	case models.Counter:
		if metric.Delta == nil {
			return "", fmt.Errorf("counter %s has no delta", metric.ID)
		}

		return strconv.FormatInt(*metric.Delta, 10), nil
	default:
		return "", fmt.Errorf("unknown metric type %q", metric.MType)
	}
}

func normalizeBaseURL(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return strings.TrimSuffix(address, "/")
	}

	return "http://" + strings.TrimSuffix(address, "/")
}
