package agent

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/mailru/easyjson"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

func testAgentConfig(address string) config.AgentConfig {
	return config.AgentConfig{
		Collector: config.CollectorConfig{PollInterval: time.Second},
		Sender:    config.SenderConfig{Address: address, ReportInterval: time.Second},
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{name: "host and port", address: "localhost:8080", want: "http://localhost:8080"},
		{name: "with scheme", address: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "trailing slash", address: "localhost:8080/", want: "http://localhost:8080"},
		{name: "https scheme", address: "https://example.com/", want: "https://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBaseURL(tt.address); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

type recordedRequest struct {
	method          string
	path            string
	contentType     string
	contentEncoding string
	metrics         models.MetricsList
	decodeErr       error
}

func newRecordingServer(t *testing.T) (*httptest.Server, func() []recordedRequest) {
	t.Helper()

	var (
		mu       sync.Mutex
		recorded []recordedRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := recordedRequest{
			method:          r.Method,
			path:            r.URL.Path,
			contentType:     r.Header.Get("Content-Type"),
			contentEncoding: r.Header.Get("Content-Encoding"),
		}

		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			req.decodeErr = err
		} else {
			req.decodeErr = easyjson.UnmarshalFromReader(zr, &req.metrics)
		}

		mu.Lock()
		recorded = append(recorded, req)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()

		return append([]recordedRequest(nil), recorded...)
	}
}

func TestAgentReportSendsAllMetricsInOneBatch(t *testing.T) {
	srv, requests := newRecordingServer(t)

	a := New(testAgentConfig(srv.URL), zap.NewNop())

	a.collector.Poll()
	a.Report(context.Background())

	recorded := requests()
	if len(recorded) != 1 {
		t.Fatalf("got %d requests, want exactly one batch", len(recorded))
	}

	req := recorded[0]

	if req.method != http.MethodPost || req.path != "/updates/" {
		t.Errorf("got %s %s, want POST /updates/", req.method, req.path)
	}

	if req.contentType != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", req.contentType)
	}

	if req.contentEncoding != "gzip" {
		t.Errorf("got Content-Encoding %q, want gzip", req.contentEncoding)
	}

	if req.decodeErr != nil {
		t.Fatalf("request body is not a valid metrics list: %v", req.decodeErr)
	}

	want := snapshotMap(t, a.collector.Snapshot())
	if len(req.metrics) != len(want) {
		t.Errorf("got %d metrics in the batch, want %d", len(req.metrics), len(want))
	}

	seen := make(map[string]bool, len(req.metrics))

	for _, m := range req.metrics {
		seen[m.ID] = true

		switch m.MType {
		case models.Gauge:
			if m.Value == nil || m.Delta != nil {
				t.Errorf("gauge %s must carry only value: %+v", m.ID, m)
			}
		case models.Counter:
			if m.Delta == nil || m.Value != nil {
				t.Errorf("counter %s must carry only delta: %+v", m.ID, m)
			}
		default:
			t.Errorf("metric %s has unexpected type %q", m.ID, m.MType)
		}
	}

	for id := range want {
		if !seen[id] {
			t.Errorf("metric %s was not reported", id)
		}
	}
}

func TestAgentReportSendsPollCountDelta(t *testing.T) {
	srv, requests := newRecordingServer(t)

	a := New(testAgentConfig(srv.URL), zap.NewNop())

	for range 3 {
		for range 5 {
			a.collector.Poll()
		}

		a.Report(context.Background())
	}

	var deltas []int64

	for _, req := range requests() {
		for _, m := range req.metrics {
			if m.ID == pollCountMetric && m.Delta != nil {
				deltas = append(deltas, *m.Delta)
			}
		}
	}

	want := []int64{5, 5, 5}
	if !reflect.DeepEqual(deltas, want) {
		t.Errorf("got PollCount deltas %v, want %v", deltas, want)
	}
}

func TestAgentReportKeepsPollCountOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := New(testAgentConfig(srv.URL), zap.NewNop())

	a.collector.Poll()
	a.Report(context.Background())
	a.collector.Poll()

	metric := snapshotMap(t, a.collector.Snapshot())[pollCountMetric]
	if metric.Delta == nil || *metric.Delta != 2 {
		t.Errorf("got PollCount %v, want 2 (неудачный репорт не должен сбрасывать счётчик)", metric.Delta)
	}
}

func TestAgentReportSurvivesServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	core, logs := observer.New(zap.InfoLevel)
	a := New(testAgentConfig(srv.URL), zap.New(core))
	a.collector.Poll()

	a.Report(context.Background())

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("got %d log entries, want one per failed batch", len(entries))
	}

	if entries[0].Level != zap.ErrorLevel {
		t.Errorf("got level %s, want error", entries[0].Level)
	}

	if count, _ := entries[0].ContextMap()["count"].(int64); count != int64(len(a.collector.Snapshot())) {
		t.Errorf("log entry must contain the number of unsent metrics, got %v", entries[0].ContextMap()["count"])
	}
}

func TestSenderSkipsEmptyBatch(t *testing.T) {
	srv, requests := newRecordingServer(t)

	s := NewSender(config.SenderConfig{Address: srv.URL, ReportInterval: time.Second}, zap.NewNop())

	if err := s.SendBatch(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := len(requests()); n != 0 {
		t.Errorf("got %d requests, empty batch must not be sent", n)
	}
}

func TestAgentRunStopsOnContextCancel(t *testing.T) {
	srv, requests := newRecordingServer(t)

	a := New(config.AgentConfig{
		Collector: config.CollectorConfig{PollInterval: 10 * time.Millisecond},
		Sender:    config.SenderConfig{Address: srv.URL, ReportInterval: 20 * time.Millisecond},
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	go func() {
		a.Run(ctx)
		close(done)
	}()

	deadline := time.After(5 * time.Second)
	for len(requests()) == 0 {
		select {
		case <-deadline:
			t.Fatal("agent did not report metrics while running")
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}

	if metric := snapshotMap(t, requests()[0].metrics)[pollCountMetric]; metric.Delta == nil || *metric.Delta == 0 {
		t.Error("agent did not poll metrics while running")
	}
}
