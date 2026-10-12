package service

import (
	"errors"
	"reflect"
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
)

type mockStorage struct {
	gauges   map[string]float64
	counters map[string]int64
}

func newMockStorage() *mockStorage {
	return &mockStorage{gauges: map[string]float64{}, counters: map[string]int64{}}
}

func (m *mockStorage) SetGauge(name string, value float64) { m.gauges[name] = value }

func (m *mockStorage) AddCounter(name string, delta int64) int64 {
	m.counters[name] += delta

	return m.counters[name]
}

func (m *mockStorage) Gauge(name string) (float64, bool) {
	v, ok := m.gauges[name]

	return v, ok
}

func (m *mockStorage) Counter(name string) (int64, bool) {
	v, ok := m.counters[name]

	return v, ok
}

func (m *mockStorage) All() []models.Metrics { return nil }

var (
	_ Storage = (*mockStorage)(nil)
	_ Storage = (*repository.MemStorage)(nil)
)

func TestMetricsServiceUpdate(t *testing.T) {
	tests := []struct {
		name    string
		mType   string
		metric  string
		value   string
		wantErr error
	}{
		{name: "gauge", mType: models.Gauge, metric: "Alloc", value: "12.5"},
		{name: "counter", mType: models.Counter, metric: "PollCount", value: "5"},
		{name: "empty name", mType: models.Gauge, metric: "", value: "1", wantErr: ErrEmptyName},
		{name: "unknown type", mType: "histogram", metric: "x", value: "1", wantErr: ErrUnknownType},
		{name: "bad gauge value", mType: models.Gauge, metric: "Alloc", value: "abc", wantErr: ErrInvalidValue},
		{name: "bad counter value", mType: models.Counter, metric: "PollCount", value: "1.5", wantErr: ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewMetricsService(newMockStorage())

			err := s.Update(tt.mType, tt.metric, tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetricsServiceCounterAccumulates(t *testing.T) {
	storage := newMockStorage()
	s := NewMetricsService(storage)

	for range 3 {
		if err := s.Update(models.Counter, "PollCount", "2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if got := storage.counters["PollCount"]; got != 6 {
		t.Errorf("got %d, want 6", got)
	}
}

func TestMetricsServiceValue(t *testing.T) {
	storage := newMockStorage()
	storage.gauges["Alloc"] = 12.5
	storage.counters["PollCount"] = 7

	s := NewMetricsService(storage)

	tests := []struct {
		name    string
		mType   string
		metric  string
		want    string
		wantErr bool
	}{
		{name: "gauge", mType: models.Gauge, metric: "Alloc", want: "12.5"},
		{name: "counter", mType: models.Counter, metric: "PollCount", want: "7"},
		{name: "unknown gauge", mType: models.Gauge, metric: "nope", wantErr: true},
		{name: "unknown counter", mType: models.Counter, metric: "nope", wantErr: true},
		{name: "unknown type", mType: "histogram", metric: "Alloc", wantErr: true},
		{name: "empty name", mType: models.Gauge, metric: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Value(tt.mType, tt.metric)
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

func TestMetricsServiceAll(t *testing.T) {
	s := NewMetricsService(repository.NewMemStorage())

	if err := s.Update(models.Gauge, "Alloc", "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(s.All()) != 1 {
		t.Errorf("got %d metrics, want 1", len(s.All()))
	}
}

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }

func TestMetricsServiceUpdateMetric(t *testing.T) {
	tests := []struct {
		name    string
		metric  models.Metrics
		want    models.Metrics
		wantErr error
	}{
		{
			name:   "gauge",
			metric: models.Metrics{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(12.5)},
			want:   models.Metrics{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(12.5)},
		},
		{
			name:   "counter returns accumulated total",
			metric: models.Metrics{ID: "PollCount", MType: models.Counter, Delta: ptrInt(5)},
			want:   models.Metrics{ID: "PollCount", MType: models.Counter, Delta: ptrInt(15)},
		},
		{
			name:    "empty name",
			metric:  models.Metrics{MType: models.Gauge, Value: ptrFloat(1)},
			wantErr: ErrEmptyName,
		},
		{
			name:    "unknown type",
			metric:  models.Metrics{ID: "x", MType: "histogram", Value: ptrFloat(1)},
			wantErr: ErrUnknownType,
		},
		{
			name:    "gauge without value",
			metric:  models.Metrics{ID: "Alloc", MType: models.Gauge, Delta: ptrInt(1)},
			wantErr: ErrInvalidValue,
		},
		{
			name:    "counter without delta",
			metric:  models.Metrics{ID: "PollCount", MType: models.Counter, Value: ptrFloat(1)},
			wantErr: ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := newMockStorage()
			storage.counters["PollCount"] = 10

			got, err := NewMetricsService(storage).UpdateMetric(tt.metric)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMetricsServiceMetric(t *testing.T) {
	storage := newMockStorage()
	storage.gauges["Alloc"] = 12.5
	storage.counters["PollCount"] = 7

	s := NewMetricsService(storage)

	tests := []struct {
		name    string
		mType   string
		metric  string
		want    models.Metrics
		wantErr error
	}{
		{
			name:  "gauge",
			mType: models.Gauge, metric: "Alloc",
			want: models.Metrics{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(12.5)},
		},
		{
			name:  "counter",
			mType: models.Counter, metric: "PollCount",
			want: models.Metrics{ID: "PollCount", MType: models.Counter, Delta: ptrInt(7)},
		},
		{name: "unknown gauge", mType: models.Gauge, metric: "nope", wantErr: ErrNotFound},
		{name: "unknown counter", mType: models.Counter, metric: "nope", wantErr: ErrNotFound},
		{name: "unknown type", mType: "histogram", metric: "Alloc", wantErr: ErrUnknownType},
		{name: "empty name", mType: models.Gauge, metric: "", wantErr: ErrEmptyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Metric(tt.mType, tt.metric)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
