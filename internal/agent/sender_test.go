package agent

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/retry"
)

type flakyTransport struct {
	failures int32
	calls    atomic.Int32
	next     http.RoundTripper
}

func (f *flakyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if f.calls.Add(1) <= f.failures {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	}

	return f.next.RoundTrip(r)
}

func newTestSender(t *testing.T, address string, transport http.RoundTripper) (*Sender, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zap.InfoLevel)

	s := NewSender(config.SenderConfig{Address: address, ReportInterval: time.Second}, zap.New(core))
	retryTransport(s).Policy.Delays = []time.Duration{time.Millisecond, 3 * time.Millisecond, 5 * time.Millisecond}

	if transport != nil {
		retryTransport(s).Next = transport
	}

	return s, logs
}

func retryTransport(s *Sender) *retry.Transport {
	return s.client.Transport.(*retry.Transport)
}

var testBatch = []models.Metrics{{ID: pollCountMetric, MType: models.Counter, Delta: new(int64)}}

func TestSenderRetriesConnectionErrors(t *testing.T) {
	srv, requests := newRecordingServer(t)
	transport := &flakyTransport{failures: 2, next: http.DefaultTransport}
	s, logs := newTestSender(t, srv.URL, transport)

	if err := s.SendBatch(context.Background(), testBatch); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := transport.calls.Load(); got != 3 {
		t.Errorf("got %d attempts, want 3", got)
	}

	if n := len(requests()); n != 1 {
		t.Errorf("server got %d batches, want 1", n)
	}

	if n := logs.FilterLevelExact(zap.WarnLevel).Len(); n != 2 {
		t.Errorf("got %d retry warnings, want 2", n)
	}
}

func TestSenderGivesUpAfterThreeRetries(t *testing.T) {
	transport := &flakyTransport{failures: 100}
	s, _ := newTestSender(t, "localhost:1", transport)

	err := s.SendBatch(context.Background(), testBatch)

	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("got error %v, want a wrapped *net.OpError", err)
	}

	if got := transport.calls.Load(); got != 4 {
		t.Errorf("got %d attempts, want 1 + 3 retries", got)
	}
}

func TestSenderDoesNotRetryServerErrors(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s, _ := newTestSender(t, srv.URL, nil)

	err := s.SendBatch(context.Background(), testBatch)

	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("got error %v, want *StatusError with code 500", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("got %d requests, a server error must not be retried", got)
	}
}

func TestIsConnectionErrorOnRefusedConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := ln.Addr().String()
	_ = ln.Close()

	s, _ := newTestSender(t, addr, nil)

	err = s.post(context.Background(), nil)
	if err == nil {
		t.Fatal("expected a connection error")
	}

	if !isConnectionError(err) {
		t.Errorf("refused connection must be retriable: %v", err)
	}

	if isConnectionError(&StatusError{StatusCode: http.StatusServiceUnavailable}) {
		t.Error("HTTP status errors must not be treated as connection errors")
	}
}

func TestSenderStopsRetryingOnShutdown(t *testing.T) {
	transport := &flakyTransport{failures: 100}
	s, _ := newTestSender(t, "localhost:1", transport)
	retryTransport(s).Policy.Delays = []time.Duration{time.Hour}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()

	if err := s.SendBatch(ctx, testBatch); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got error %v, want context.DeadlineExceeded", err)
	}

	if time.Since(start) > time.Second {
		t.Error("SendBatch must stop waiting when the context is done")
	}
}
