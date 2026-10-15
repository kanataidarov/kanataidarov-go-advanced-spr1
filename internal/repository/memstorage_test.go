package repository

import (
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

func mustGauge(t *testing.T, s *MemStorage, name string) (float64, bool) {
	t.Helper()

	v, ok, err := s.Gauge(t.Context(), name)
	if err != nil {
		t.Fatalf("gauge %q: %v", name, err)
	}

	return v, ok
}

func mustCounter(t *testing.T, s *MemStorage, name string) (int64, bool) {
	t.Helper()

	v, ok, err := s.Counter(t.Context(), name)
	if err != nil {
		t.Fatalf("counter %q: %v", name, err)
	}

	return v, ok
}

func mustSetGauge(t *testing.T, s *MemStorage, name string, value float64) {
	t.Helper()

	if err := s.SetGauge(t.Context(), name, value); err != nil {
		t.Fatalf("set gauge %q: %v", name, err)
	}
}

func mustAddCounter(t *testing.T, s *MemStorage, name string, delta int64) int64 {
	t.Helper()

	total, err := s.AddCounter(t.Context(), name, delta)
	if err != nil {
		t.Fatalf("add counter %q: %v", name, err)
	}

	return total
}

func TestMemStorageGauge(t *testing.T) {
	s := NewMemStorage()

	if _, ok := mustGauge(t, s, "missing"); ok {
		t.Error("expected missing gauge")
	}

	mustSetGauge(t, s, "Alloc", 1.5)
	mustSetGauge(t, s, "Alloc", 2.5)

	got, ok := mustGauge(t, s, "Alloc")
	if !ok || got != 2.5 {
		t.Errorf("got (%v, %v), want (2.5, true)", got, ok)
	}
}

func TestMemStorageCounter(t *testing.T) {
	s := NewMemStorage()

	if _, ok := mustCounter(t, s, "missing"); ok {
		t.Error("expected missing counter")
	}

	if got := mustAddCounter(t, s, "PollCount", 5); got != 5 {
		t.Errorf("got %d, want 5", got)
	}

	if got := mustAddCounter(t, s, "PollCount", 7); got != 12 {
		t.Errorf("got %d, want 12", got)
	}

	got, ok := mustCounter(t, s, "PollCount")
	if !ok || got != 12 {
		t.Errorf("got (%v, %v), want (12, true)", got, ok)
	}
}

func TestMemStorageAll(t *testing.T) {
	s := NewMemStorage()
	mustSetGauge(t, s, "Alloc", 1)
	mustAddCounter(t, s, "PollCount", 3)

	all, err := s.All(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("got %d metrics, want 2", len(all))
	}

	byID := make(map[string]models.Metrics, len(all))
	for _, m := range all {
		byID[m.ID] = m
	}

	if m := byID["Alloc"]; m.MType != models.Gauge || m.Value == nil || *m.Value != 1 {
		t.Errorf("unexpected gauge metric: %+v", m)
	}

	if m := byID["PollCount"]; m.MType != models.Counter || m.Delta == nil || *m.Delta != 3 {
		t.Errorf("unexpected counter metric: %+v", m)
	}
}

func TestMemStorageRestore(t *testing.T) {
	s := NewMemStorage()
	mustSetGauge(t, s, "Old", 1)
	mustAddCounter(t, s, "PollCount", 100)

	err := s.Restore([]models.Metrics{
		{ID: "LastGC", MType: models.Gauge, Value: ptrFloat(1257894000000000000)},
		{ID: "PollCount", MType: models.Counter, Delta: ptrInt(42)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := mustGauge(t, s, "Old"); ok {
		t.Error("restore must replace previous contents")
	}

	if v, ok := mustGauge(t, s, "LastGC"); !ok || v != 1257894000000000000 {
		t.Errorf("got gauge (%v, %v)", v, ok)
	}

	if v, ok := mustCounter(t, s, "PollCount"); !ok || v != 42 {
		t.Errorf("got counter (%v, %v), want (42, true): restore sets values, not adds", v, ok)
	}
}

func TestMemStorageRestoreRejectsInvalidMetrics(t *testing.T) {
	cases := []models.Metrics{
		{MType: models.Gauge, Value: ptrFloat(1)},
		{ID: "A", MType: models.Gauge},
		{ID: "B", MType: models.Counter, Value: ptrFloat(1)},
		{ID: "C", MType: "histogram", Value: ptrFloat(1)},
	}

	for _, m := range cases {
		s := NewMemStorage()
		mustSetGauge(t, s, "Keep", 1)

		if err := s.Restore([]models.Metrics{m}); err == nil {
			t.Errorf("%+v: expected an error", m)
		}

		if _, ok := mustGauge(t, s, "Keep"); !ok {
			t.Errorf("%+v: failed restore must not change the storage", m)
		}
	}
}

func TestMemStorageUpdateMetrics(t *testing.T) {
	s := NewMemStorage()
	mustAddCounter(t, s, "PollCount", 10)

	err := s.UpdateMetrics(t.Context(), []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1)},
		{ID: "PollCount", MType: models.Counter, Delta: ptrInt(5)},
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(2)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v, ok := mustGauge(t, s, "Alloc"); !ok || v != 2 {
		t.Errorf("got gauge (%v, %v), want (2, true)", v, ok)
	}

	if v, ok := mustCounter(t, s, "PollCount"); !ok || v != 15 {
		t.Errorf("got counter (%v, %v), want (15, true)", v, ok)
	}
}

func TestMemStorageUpdateMetricsIsAtomic(t *testing.T) {
	s := NewMemStorage()

	err := s.UpdateMetrics(t.Context(), []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1)},
		{ID: "Broken", MType: models.Counter},
	})
	if err == nil {
		t.Fatal("expected an error for an invalid metric")
	}

	if _, ok := mustGauge(t, s, "Alloc"); ok {
		t.Error("a failed batch must not change the storage")
	}
}
