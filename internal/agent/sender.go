package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mailru/easyjson"
	"go.uber.org/zap"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/retry"
)

const requestTimeout = 15 * time.Second

type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status code %d", e.StatusCode)
}

type Sender struct {
	client         *http.Client
	baseURL        string
	reportInterval time.Duration
}

func NewSender(cfg config.SenderConfig, log *zap.Logger) *Sender {
	onRetry := func(err error, attempt int, delay time.Duration) {
		log.Warn("cannot connect to server, retrying",
			zap.Int("attempt", attempt), zap.Duration("delay", delay), zap.Error(err))
	}

	return &Sender{
		client: &http.Client{
			Timeout:   requestTimeout,
			Transport: &retry.Transport{Policy: retry.New(isConnectionError, onRetry)},
		},
		baseURL:        normalizeBaseURL(cfg.Address),
		reportInterval: cfg.ReportInterval,
	}
}

func isConnectionError(err error) bool {
	var opErr *net.OpError

	return errors.As(err, &opErr) && opErr.Op == "dial"
}

func (s *Sender) ReportInterval() time.Duration {
	return s.reportInterval
}

func (s *Sender) SendBatch(ctx context.Context, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	body, err := easyjson.Marshal(models.MetricsList(metrics))
	if err != nil {
		return fmt.Errorf("cannot encode metrics: %w", err)
	}

	compressed, err := gzipBytes(body)
	if err != nil {
		return fmt.Errorf("cannot compress metrics: %w", err)
	}

	return s.post(ctx, compressed)
}

func (s *Sender) post(ctx context.Context, gzipped []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/updates/", bytes.NewReader(gzipped))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return &StatusError{StatusCode: resp.StatusCode}
	}

	return nil
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)

	if _, err := zw.Write(data); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func normalizeBaseURL(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return strings.TrimSuffix(address, "/")
	}

	return "http://" + strings.TrimSuffix(address, "/")
}
