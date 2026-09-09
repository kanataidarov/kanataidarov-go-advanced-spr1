package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

type stubService struct {
	updateCalls [][3]string
	updateErr   error
	value       string
	valueErr    error
	all         []models.Metrics
}

func (s *stubService) Update(mType, name, rawValue string) error {
	s.updateCalls = append(s.updateCalls, [3]string{mType, name, rawValue})

	return s.updateErr
}

func (s *stubService) Value(_, _ string) (string, error) {
	return s.value, s.valueErr
}

func (s *stubService) All() []models.Metrics {
	return s.all
}

func TestHandlerPassesUpdateParamsToService(t *testing.T) {
	stub := &stubService{}
	router := NewMetricsHandler(stub).Router()

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
			router := NewMetricsHandler(&stubService{valueErr: tt.err}).Router()

			req := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", http.NoBody)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestHandlerIndexSortsMetricsByName(t *testing.T) {
	stub := &stubService{all: []models.Metrics{
		{ID: "Zeta", MType: models.Gauge, Value: ptrFloat(2)},
		{ID: "Alpha", MType: models.Counter, Delta: ptrInt(7)},
	}}
	router := NewMetricsHandler(stub).Router()

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
	router := NewMetricsHandler(stub).Router()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "<script>") {
		t.Errorf("metric name was not escaped:\n%s", rec.Body.String())
	}
}

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }
