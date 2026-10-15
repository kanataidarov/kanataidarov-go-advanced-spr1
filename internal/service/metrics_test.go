package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
)

type mockStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	err      error
	batches  [][]models.Metrics
}

func newMockStorage() *mockStorage {
	return &mockStorage{gauges: map[string]float64{}, counters: map[string]int64{}}
}

func (m *mockStorage) SetGauge(_ context.Context, name string, value float64) error {
	if m.err != nil {
		return m.err
	}

	m.gauges[name] = value

	return nil
}

func (m *mockStorage) AddCounter(_ context.Context, name string, delta int64) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}

	m.counters[name] += delta

	return m.counters[name], nil
}

func (m *mockStorage) Gauge(_ context.Context, name string) (float64, bool, error) {
	v, ok := m.gauges[name]

	return v, ok, m.err
}

func (m *mockStorage) Counter(_ context.Context, name string) (int64, bool, error) {
	v, ok := m.counters[name]

	return v, ok, m.err
}

func (m *mockStorage) All(_ context.Context) ([]models.Metrics, error) { return nil, m.err }

func (m *mockStorage) UpdateMetrics(_ context.Context, metrics []models.Metrics) error {
	if m.err != nil {
		return m.err
	}

	m.batches = append(m.batches, metrics)

	return nil
}

var (
	_ Storage = (*mockStorage)(nil)
	_ Storage = (*repository.MemStorage)(nil)
	_ Storage = (*repository.PersistentStorage)(nil)
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

			err := s.Update(t.Context(), tt.mType, tt.metric, tt.value)
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
		if err := s.Update(t.Context(), models.Counter, "PollCount", "2"); err != nil {
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
			got, err := s.Value(t.Context(), tt.mType, tt.metric)
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

	if err := s.Update(t.Context(), models.Gauge, "Alloc", "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all, err := s.All(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(all) != 1 {
		t.Errorf("got %d metrics, want 1", len(all))
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

			got, err := NewMetricsService(storage).UpdateMetric(t.Context(), tt.metric)
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
			got, err := s.Metric(t.Context(), tt.mType, tt.metric)
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

func TestMetricsServicePropagatesStorageErrors(t *testing.T) {
	errDB := errors.New("connection refused")

	storage := newMockStorage()
	storage.err = errDB

	s := NewMetricsService(storage)
	ctx := t.Context()

	checks := map[string]error{
		"Update gauge":   s.Update(ctx, models.Gauge, "Alloc", "1"),
		"Update counter": s.Update(ctx, models.Counter, "PollCount", "1"),
	}

	_, checks["UpdateMetric gauge"] = s.UpdateMetric(ctx, models.Metrics{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1)})
	_, checks["UpdateMetric counter"] = s.UpdateMetric(ctx, models.Metrics{ID: "PollCount", MType: models.Counter, Delta: ptrInt(1)})
	_, checks["Metric"] = s.Metric(ctx, models.Gauge, "Alloc")
	_, checks["Value"] = s.Value(ctx, models.Counter, "PollCount")
	_, checks["All"] = s.All(ctx)
	checks["UpdateMetrics"] = s.UpdateMetrics(ctx, []models.Metrics{{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1)}})

	for name, err := range checks {
		if !errors.Is(err, errDB) {
			t.Errorf("%s: got error %v, want wrapped %v", name, err, errDB)
		}

		for _, known := range []error{ErrEmptyName, ErrUnknownType, ErrInvalidValue, ErrNotFound} {
			if errors.Is(err, known) {
				t.Errorf("%s: storage error must not look like %v", name, known)
			}
		}
	}
}

func TestMetricsServiceUpdateMetricsMergesAndSortsBatch(t *testing.T) {
	storage := newMockStorage()

	err := NewMetricsService(storage).UpdateMetrics(t.Context(), []models.Metrics{
		{ID: "PollCount", MType: models.Counter, Delta: ptrInt(3)},
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1)},
		{ID: "PollCount", MType: models.Counter, Delta: ptrInt(4)},
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(2)},
		{ID: "Alloc", MType: models.Counter, Delta: ptrInt(1)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]models.Metrics{{
		{ID: "Alloc", MType: models.Counter, Delta: ptrInt(1)},
		{ID: "PollCount", MType: models.Counter, Delta: ptrInt(7)},
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(2)},
	}}
	if !reflect.DeepEqual(storage.batches, want) {
		t.Errorf("got batches %+v, want %+v", storage.batches, want)
	}
}

func TestMetricsServiceUpdateMetricsRejectsInvalidBatch(t *testing.T) {
	valid := models.Metrics{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1)}

	tests := []struct {
		name    string
		batch   []models.Metrics
		wantErr error
	}{
		{name: "nil", batch: nil, wantErr: ErrEmptyBatch},
		{name: "empty", batch: []models.Metrics{}, wantErr: ErrEmptyBatch},
		{name: "empty name", batch: []models.Metrics{valid, {MType: models.Gauge, Value: ptrFloat(1)}}, wantErr: ErrEmptyName},
		{name: "unknown type", batch: []models.Metrics{valid, {ID: "x", MType: "histogram", Value: ptrFloat(1)}}, wantErr: ErrUnknownType},
		{name: "gauge without value", batch: []models.Metrics{valid, {ID: "x", MType: models.Gauge, Delta: ptrInt(1)}}, wantErr: ErrInvalidValue},
		{name: "counter without delta", batch: []models.Metrics{valid, {ID: "x", MType: models.Counter}}, wantErr: ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := newMockStorage()

			err := NewMetricsService(storage).UpdateMetrics(t.Context(), tt.batch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}

			if len(storage.batches) != 0 {
				t.Error("invalid batch must not reach the storage")
			}
		})
	}
}
