package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mailru/easyjson"
	"go.uber.org/zap"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

func newTestRouter() (http.Handler, *repository.MemStorage) {
	storage := repository.NewMemStorage()
	h := NewMetricsHandler(service.NewMetricsService(storage), zap.NewNop())

	return h.Router(), storage
}

func seedStorage(t *testing.T, s *repository.MemStorage) {
	t.Helper()

	if err := s.SetGauge(context.Background(), "Alloc", 12.5); err != nil {
		t.Fatalf("seed gauge: %v", err)
	}

	if _, err := s.AddCounter(context.Background(), "PollCount", 7); err != nil {
		t.Fatalf("seed counter: %v", err)
	}
}

func storedGauge(t *testing.T, s *repository.MemStorage, name string) (float64, bool) {
	t.Helper()

	v, ok, err := s.Gauge(context.Background(), name)
	if err != nil {
		t.Fatalf("read gauge %q: %v", name, err)
	}

	return v, ok
}

func storedCounter(t *testing.T, s *repository.MemStorage, name string) (int64, bool) {
	t.Helper()

	v, ok, err := s.Counter(context.Background(), name)
	if err != nil {
		t.Fatalf("read counter %q: %v", name, err)
	}

	return v, ok
}

func storedCount(t *testing.T, s *repository.MemStorage) int {
	t.Helper()

	all, err := s.All(context.Background())
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}

	return len(all)
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
		{name: "root", method: http.MethodPost, path: "/", wantStatus: http.StatusMethodNotAllowed},
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

	if got, ok := storedGauge(t, storage, "Alloc"); !ok || got != 20 {
		t.Errorf("got gauge (%v, %v), want (20, true)", got, ok)
	}

	if got, ok := storedCounter(t, storage, "PollCount"); !ok || got != 12 {
		t.Errorf("got counter (%v, %v), want (12, true)", got, ok)
	}

	if storedCount(t, storage) != 2 {
		t.Errorf("got %d stored metrics, want 2", storedCount(t, storage))
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

func TestMetricsHandlerValue(t *testing.T) {
	router, storage := newTestRouter()
	seedStorage(t, storage)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "gauge", path: "/value/gauge/Alloc", wantStatus: http.StatusOK, wantBody: "12.5"},
		{name: "counter", path: "/value/counter/PollCount", wantStatus: http.StatusOK, wantBody: "7"},
		{name: "unknown metric", path: "/value/gauge/Nope", wantStatus: http.StatusNotFound},
		{name: "unknown type", path: "/value/histogram/Alloc", wantStatus: http.StatusBadRequest},
		{name: "without name", path: "/value/gauge", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, http.NoBody))

			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("got body %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestMetricsHandlerIndex(t *testing.T) {
	router, storage := newTestRouter()
	seedStorage(t, storage)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("got Content-Type %q, want text/html; charset=utf-8", got)
	}

	body := rec.Body.String()
	for _, want := range []string{"<!DOCTYPE html>", "<html", "</html>", "Alloc", "12.5", "PollCount", "7"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func postJSON(t *testing.T, router http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func decodeMetric(t *testing.T, rec *httptest.ResponseRecorder) models.Metrics {
	t.Helper()

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", got)
	}

	var m models.Metrics
	if err := easyjson.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("cannot decode response %q: %v", rec.Body.String(), err)
	}

	return m
}

func TestMetricsHandlerUpdateJSON(t *testing.T) {
	for _, path := range []string{"/update", "/update/"} {
		t.Run(path, func(t *testing.T) {
			router, storage := newTestRouter()

			rec := postJSON(t, router, path, `{"id":"LastGC","type":"gauge","value":1744184459}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("gauge: got status %d, want 200: %s", rec.Code, rec.Body.String())
			}

			if got := decodeMetric(t, rec); got.ID != "LastGC" || got.MType != models.Gauge ||
				got.Value == nil || *got.Value != 1744184459 || got.Delta != nil {
				t.Errorf("gauge: got response %+v", got)
			}

			if v, ok := storedGauge(t, storage, "LastGC"); !ok || v != 1744184459 {
				t.Errorf("gauge: got stored (%v, %v), want (1744184459, true)", v, ok)
			}

			for i, wantTotal := range []int64{5, 12} {
				delta := []string{"5", "7"}[i]

				rec = postJSON(t, router, path, `{"id":"PollCount","type":"counter","delta":`+delta+`}`)
				if rec.Code != http.StatusOK {
					t.Fatalf("counter: got status %d, want 200: %s", rec.Code, rec.Body.String())
				}

				if got := decodeMetric(t, rec); got.Delta == nil || *got.Delta != wantTotal {
					t.Errorf("counter: got response %+v, want delta %d", got, wantTotal)
				}
			}
		})
	}
}

func TestMetricsHandlerUpdateJSONErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "empty body", body: "", wantStatus: http.StatusBadRequest},
		{name: "malformed JSON", body: `{"id":"Alloc"`, wantStatus: http.StatusBadRequest},
		{name: "wrong value type", body: `{"id":"Alloc","type":"gauge","value":"12"}`, wantStatus: http.StatusBadRequest},
		{name: "unknown type", body: `{"id":"Alloc","type":"histogram","value":1}`, wantStatus: http.StatusBadRequest},
		{name: "gauge without value", body: `{"id":"Alloc","type":"gauge"}`, wantStatus: http.StatusBadRequest},
		{name: "counter without delta", body: `{"id":"PollCount","type":"counter","value":1}`, wantStatus: http.StatusBadRequest},
		{name: "without name", body: `{"type":"gauge","value":1}`, wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, storage := newTestRouter()

			rec := postJSON(t, router, "/update/", tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if n := storedCount(t, storage); n != 0 {
				t.Errorf("got %d stored metrics, want 0", n)
			}
		})
	}
}

func TestMetricsHandlerValueJSON(t *testing.T) {
	router, storage := newTestRouter()
	seedStorage(t, storage)

	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
		want       models.Metrics
	}{
		{
			name:       "gauge",
			path:       "/value/",
			body:       `{"id":"Alloc","type":"gauge"}`,
			wantStatus: http.StatusOK,
			want:       models.Metrics{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(12.5)},
		},
		{
			name:       "counter without trailing slash",
			path:       "/value",
			body:       `{"id":"PollCount","type":"counter"}`,
			wantStatus: http.StatusOK,
			want:       models.Metrics{ID: "PollCount", MType: models.Counter, Delta: ptrInt(7)},
		},
		{name: "unknown metric", path: "/value/", body: `{"id":"Nope","type":"gauge"}`, wantStatus: http.StatusNotFound},
		{name: "unknown type", path: "/value/", body: `{"id":"Alloc","type":"histogram"}`, wantStatus: http.StatusBadRequest},
		{name: "without name", path: "/value/", body: `{"type":"gauge"}`, wantStatus: http.StatusNotFound},
		{name: "malformed JSON", path: "/value/", body: `{"id":`, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := postJSON(t, router, tt.path, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			if got := decodeMetric(t, rec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMetricsHandlerJSONAndURLShareStorage(t *testing.T) {
	router, _ := newTestRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/3", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	rec = postJSON(t, router, "/update/", `{"id":"PollCount","type":"counter","delta":4}`)
	if got := decodeMetric(t, rec); got.Delta == nil || *got.Delta != 7 {
		t.Fatalf("got %+v, want delta 7", got)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/value/counter/PollCount", http.NoBody))

	if rec.Body.String() != "7" {
		t.Errorf("got %q, want 7", rec.Body.String())
	}
}
