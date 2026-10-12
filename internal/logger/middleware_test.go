package logger

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		handler    http.HandlerFunc
		wantStatus int64
		wantSize   int64
		wantBody   string
	}{
		{
			name:   "explicit status and body",
			method: http.MethodPost,
			target: "/update/counter/PollCount/5",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(w, "hello")
			},
			wantStatus: http.StatusCreated,
			wantSize:   5,
			wantBody:   "hello",
		},
		{
			name:   "write without WriteHeader means 200",
			method: http.MethodGet,
			target: "/value/gauge/Alloc?x=1",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "12.5")
				_, _ = io.WriteString(w, "!!")
			},
			wantStatus: http.StatusOK,
			wantSize:   6,
			wantBody:   "12.5!!",
		},
		{
			name:   "status without body",
			method: http.MethodGet,
			target: "/value/gauge/unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantSize:   0,
		},
		{
			name:       "empty handler means 200",
			method:     http.MethodGet,
			target:     "/",
			handler:    func(http.ResponseWriter, *http.Request) {},
			wantStatus: http.StatusOK,
			wantSize:   0,
		},
		{
			name:   "only the first WriteHeader counts",
			method: http.MethodPost,
			target: "/update/",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, "bad")
			},
			wantStatus: http.StatusBadRequest,
			wantSize:   3,
			wantBody:   "bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			h := RequestLogger(zap.New(core))(tt.handler)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			if int64(rec.Code) != tt.wantStatus {
				t.Errorf("client got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if rec.Body.String() != tt.wantBody {
				t.Errorf("client got body %q, want %q", rec.Body.String(), tt.wantBody)
			}

			entries := logs.All()
			if len(entries) != 1 {
				t.Fatalf("got %d log entries, want 1", len(entries))
			}

			entry := entries[0]
			if entry.Level != zap.InfoLevel {
				t.Errorf("got level %s, want info", entry.Level)
			}

			fields := entry.ContextMap()

			if got := fields["uri"]; got != tt.target {
				t.Errorf("got uri %v, want %q", got, tt.target)
			}

			if got := fields["method"]; got != tt.method {
				t.Errorf("got method %v, want %q", got, tt.method)
			}

			if got := fields["status"]; got != tt.wantStatus {
				t.Errorf("got status %v, want %d", got, tt.wantStatus)
			}

			if got := fields["size"]; got != tt.wantSize {
				t.Errorf("got size %v, want %d", got, tt.wantSize)
			}

			if d, ok := fields["duration"].(time.Duration); !ok || d < 0 {
				t.Errorf("got duration %v, want non-negative time.Duration", fields["duration"])
			}
		})
	}
}

func TestRequestLoggerKeepsFlusher(t *testing.T) {
	core, _ := observer.New(zap.InfoLevel)

	h := RequestLogger(zap.New(core))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("flush through logging writer failed: %v", err)
		}
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Error("underlying writer was not flushed")
	}
}

func TestNew(t *testing.T) {
	log, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !log.Core().Enabled(zap.InfoLevel) {
		t.Error("info level must be enabled")
	}

	if log.Core().Enabled(zap.DebugLevel) {
		t.Error("debug level must be disabled")
	}
}
