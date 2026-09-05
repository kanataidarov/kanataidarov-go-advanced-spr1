package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

func newTestRouter() (http.Handler, *repository.MemStorage) {
	storage := repository.NewMemStorage()
	h := NewMetricsHandler(service.NewMetricsService(storage))

	return h.Router(), storage
}

func TestMetricsHandlerUpdate(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "valid gauge", method: http.MethodPost, path: "/update/gauge/Alloc/12.5", wantStatus: http.StatusOK},
		{name: "valid counter", method: http.MethodPost, path: "/update/counter/PollCount/527", wantStatus: http.StatusOK},
		{name: "without name", method: http.MethodPost, path: "/update/gauge/", wantStatus: http.StatusNotFound},
		{name: "without value", method: http.MethodPost, path: "/update/gauge/Alloc", wantStatus: http.StatusNotFound},
		{name: "unknown type", method: http.MethodPost, path: "/update/histogram/Alloc/1", wantStatus: http.StatusBadRequest},
		{name: "invalid gauge value", method: http.MethodPost, path: "/update/gauge/Alloc/abc", wantStatus: http.StatusBadRequest},
		{name: "invalid counter value", method: http.MethodPost, path: "/update/counter/PollCount/1.5", wantStatus: http.StatusBadRequest},
		{name: "wrong method", method: http.MethodGet, path: "/update/gauge/Alloc/1", wantStatus: http.StatusMethodNotAllowed},
		{name: "unknown endpoint", method: http.MethodPost, path: "/unknown/gauge/Alloc/1", wantStatus: http.StatusNotFound},
		{name: "root", method: http.MethodPost, path: "/", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := newTestRouter()

			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			req.Header.Set("Content-Type", "text/plain")

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestMetricsHandlerStoresValues(t *testing.T) {
	router, storage := newTestRouter()

	requests := []string{
		"/update/gauge/Alloc/12.5",
		"/update/gauge/Alloc/20",
		"/update/counter/PollCount/5",
		"/update/counter/PollCount/7",
	}

	for _, path := range requests {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, http.NoBody))

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: got status %d, want 200", path, rec.Code)
		}
	}

	if got, ok := storage.Gauge("Alloc"); !ok || got != 20 {
		t.Errorf("got gauge (%v, %v), want (20, true)", got, ok)
	}

	if got, ok := storage.Counter("PollCount"); !ok || got != 12 {
		t.Errorf("got counter (%v, %v), want (12, true)", got, ok)
	}

	if len(storage.All()) != 2 {
		t.Errorf("got %d stored metrics, want 2", len(storage.All()))
	}
}

func TestMetricsHandlerResponseHeaders(t *testing.T) {
	router, _ := newTestRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/update/"+models.Counter+"/someMetric/527", http.NoBody))

	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("got Content-Type %q, want text/plain; charset=utf-8", got)
	}

	if rec.Body.Len() == 0 {
		t.Error("expected non-empty response body")
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/", want: 0},
		{path: "", want: 0},
		{path: "/update/gauge/Alloc/1", want: 4},
		{path: "/update/gauge/Alloc/1/", want: 4},
	}

	for _, tt := range tests {
		if got := len(splitPath(tt.path)); got != tt.want {
			t.Errorf("splitPath(%q): got %d parts, want %d", tt.path, got, tt.want)
		}
	}
}
