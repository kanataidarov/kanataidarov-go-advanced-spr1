package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }

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

func TestMetricValue(t *testing.T) {
	tests := []struct {
		name    string
		metric  models.Metrics
		want    string
		wantErr bool
	}{
		{name: "gauge", metric: models.Metrics{ID: "a", MType: models.Gauge, Value: ptrFloat(12.5)}, want: "12.5"},
		{name: "counter", metric: models.Metrics{ID: "b", MType: models.Counter, Delta: ptrInt(527)}, want: "527"},
		{name: "gauge without value", metric: models.Metrics{ID: "c", MType: models.Gauge}, wantErr: true},
		{name: "counter without delta", metric: models.Metrics{ID: "d", MType: models.Counter}, wantErr: true},
		{name: "unknown type", metric: models.Metrics{ID: "e", MType: "histogram"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := metricValue(tt.metric)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

type recordedRequest struct {
	method      string
	path        string
	contentType string
}

func TestAgentReportSendsAllMetrics(t *testing.T) {
	var (
		mu       sync.Mutex
		recorded []recordedRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		recorded = append(recorded, recordedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
		})
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := New(config.AgentConfig{
		Address:        srv.URL,
		PollInterval:   time.Second,
		ReportInterval: time.Second,
	})

	a.collector.Poll()
	a.Report(context.Background())

	mu.Lock()
	defer mu.Unlock()

	if len(recorded) != len(a.collector.Snapshot()) {
		t.Fatalf("got %d requests, want %d", len(recorded), len(a.collector.Snapshot()))
	}

	var sawPollCount bool

	for _, req := range recorded {
		if req.method != http.MethodPost {
			t.Errorf("got method %s, want POST", req.method)
		}

		if req.contentType != "text/plain" {
			t.Errorf("got Content-Type %q, want text/plain", req.contentType)
		}

		if !strings.HasPrefix(req.path, "/update/") || len(strings.Split(strings.Trim(req.path, "/"), "/")) != 4 {
			t.Errorf("unexpected request path %q", req.path)
		}

		if strings.HasPrefix(req.path, "/update/counter/PollCount/") {
			sawPollCount = true
		}
	}

	if !sawPollCount {
		t.Error("PollCount was not reported")
	}
}

func TestAgentReportSurvivesServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := New(config.AgentConfig{Address: srv.URL, PollInterval: time.Second, ReportInterval: time.Second})
	a.collector.Poll()

	a.Report(context.Background()) // не должно паниковать
}

func TestAgentRunStopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := New(config.AgentConfig{
		Address:        srv.URL,
		PollInterval:   10 * time.Millisecond,
		ReportInterval: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})

	go func() {
		a.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}

	if metric := snapshotMap(t, a.collector.Snapshot())[pollCountMetric]; metric.Delta == nil || *metric.Delta == 0 {
		t.Error("agent did not poll metrics while running")
	}
}
