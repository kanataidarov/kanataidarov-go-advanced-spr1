package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

type stubService struct {
	updateCalls       [][3]string
	updateMetricCalls []models.Metrics
	updateErr         error
	value             string
	metric            models.Metrics
	valueErr          error
	all               []models.Metrics
}

func (s *stubService) Update(mType, name, rawValue string) error {
	s.updateCalls = append(s.updateCalls, [3]string{mType, name, rawValue})

	return s.updateErr
}

func (s *stubService) Value(_, _ string) (string, error) {
	return s.value, s.valueErr
}

func (s *stubService) UpdateMetric(m models.Metrics) (models.Metrics, error) {
	s.updateMetricCalls = append(s.updateMetricCalls, m)

	return s.metric, s.updateErr
}

func (s *stubService) Metric(_, _ string) (models.Metrics, error) {
	return s.metric, s.valueErr
}

func (s *stubService) All() []models.Metrics {
	return s.all
}

func TestHandlerPassesUpdateParamsToService(t *testing.T) {
	stub := &stubService{}
	router := NewMetricsHandler(stub, zap.NewNop()).Router()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/12.5", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	want := [3]string{"gauge", "Alloc", "12.5"}
	if len(stub.updateCalls) != 1 || stub.updateCalls[0] != want {
		t.Errorf("got Update calls %v, want exactly one %v", stub.updateCalls, want)
	}
}

func TestHandlerPassesJSONMetricToService(t *testing.T) {
	stub := &stubService{metric: models.Metrics{ID: "PollCount", MType: models.Counter, Delta: ptrInt(12)}}
	router := NewMetricsHandler(stub, zap.NewNop()).Router()

	req := httptest.NewRequest(http.MethodPost, "/update/", strings.NewReader(`{"id":"PollCount","type":"counter","delta":5}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	want := models.Metrics{ID: "PollCount", MType: models.Counter, Delta: ptrInt(5)}
	if len(stub.updateMetricCalls) != 1 || !reflect.DeepEqual(stub.updateMetricCalls[0], want) {
		t.Errorf("got UpdateMetric calls %+v, want exactly one %+v", stub.updateMetricCalls, want)
	}

	if body := rec.Body.String(); body != `{"id":"PollCount","type":"counter","delta":12}` {
		t.Errorf("got body %s, want the metric returned by the service", body)
	}
}

func TestHandlerMapsServiceErrorsToStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: service.ErrNotFound, want: http.StatusNotFound},
		{name: "empty name", err: service.ErrEmptyName, want: http.StatusNotFound},
		{name: "unknown type", err: service.ErrUnknownType, want: http.StatusBadRequest},
		{name: "invalid value", err: service.ErrInvalidValue, want: http.StatusBadRequest},
		{name: "unexpected", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewMetricsHandler(&stubService{valueErr: tt.err}, zap.NewNop()).Router()

			req := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", http.NoBody)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("GET /value/gauge/Alloc: got status %d, want %d", rec.Code, tt.want)
			}

			req = httptest.NewRequest(http.MethodPost, "/value/", strings.NewReader(`{"id":"Alloc","type":"gauge"}`))
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("POST /value/: got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestHandlerIndexSortsMetricsByName(t *testing.T) {
	stub := &stubService{all: []models.Metrics{
		{ID: "Zeta", MType: models.Gauge, Value: ptrFloat(2)},
		{ID: "Alpha", MType: models.Counter, Delta: ptrInt(7)},
	}}
	router := NewMetricsHandler(stub, zap.NewNop()).Router()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()

	alpha := strings.Index(body, "Alpha")
	zeta := strings.Index(body, "Zeta")

	if alpha < 0 || zeta < 0 {
		t.Fatalf("body does not list both metrics: %q", body)
	}

	if alpha > zeta {
		t.Errorf("got Alpha after Zeta, want metrics sorted by name:\n%s", body)
	}
}

func TestHandlerIndexEscapesMetricNames(t *testing.T) {
	stub := &stubService{all: []models.Metrics{
		{ID: "<script>alert(1)</script>", MType: models.Gauge, Value: ptrFloat(1)},
	}}
	router := NewMetricsHandler(stub, zap.NewNop()).Router()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "<script>") {
		t.Errorf("metric name was not escaped:\n%s", rec.Body.String())
	}
}

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }

func TestHandlerLogsInternalErrors(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	h := NewMetricsHandler(&stubService{}, zap.New(core))

	rec := httptest.NewRecorder()
	h.writeServiceError(rec, errors.New("storage is broken"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("got %d log entries, want 1", len(entries))
	}

	if entries[0].Level != zap.ErrorLevel {
		t.Errorf("got level %s, want error", entries[0].Level)
	}

	if got := entries[0].ContextMap()["error"]; got != "storage is broken" {
		t.Errorf("got error field %v, want %q", got, "storage is broken")
	}
}

func TestHandlerDoesNotLogClientErrors(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	h := NewMetricsHandler(&stubService{}, zap.New(core))

	h.writeServiceError(httptest.NewRecorder(), service.ErrNotFound)

	if n := len(logs.All()); n != 0 {
		t.Errorf("got %d log entries, want 0: client errors are logged by the request middleware", n)
	}
}

type stubPinger struct {
	err   error
	calls int
}

func (p *stubPinger) Ping(_ context.Context) error {
	p.calls++

	return p.err
}

func TestPing(t *testing.T) {
	tests := []struct {
		name      string
		pinger    *stubPinger
		wantCode  int
		wantCalls int
	}{
		{name: "database is reachable", pinger: &stubPinger{}, wantCode: http.StatusOK, wantCalls: 1},
		{name: "database is unreachable", pinger: &stubPinger{err: errors.New("connection refused")}, wantCode: http.StatusInternalServerError, wantCalls: 1},
		{name: "database is not configured", wantCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.pinger != nil {
				opts = append(opts, WithPinger(tt.pinger))
			}

			router := NewMetricsHandler(&stubService{}, zap.NewNop(), opts...).Router()

			req := httptest.NewRequest(http.MethodGet, "/ping", http.NoBody)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantCode)
			}

			if tt.pinger != nil && tt.pinger.calls != tt.wantCalls {
				t.Errorf("got %d Ping calls, want %d", tt.pinger.calls, tt.wantCalls)
			}
		})
	}
}
