package repository

import (
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

func TestMemStorageGauge(t *testing.T) {
	s := NewMemStorage()

	if _, ok := s.Gauge("missing"); ok {
		t.Error("expected missing gauge")
	}

	s.SetGauge("Alloc", 1.5)
	s.SetGauge("Alloc", 2.5)

	got, ok := s.Gauge("Alloc")
	if !ok || got != 2.5 {
		t.Errorf("got (%v, %v), want (2.5, true)", got, ok)
	}
}

func TestMemStorageCounter(t *testing.T) {
	s := NewMemStorage()

	if _, ok := s.Counter("missing"); ok {
		t.Error("expected missing counter")
	}

	if got := s.AddCounter("PollCount", 5); got != 5 {
		t.Errorf("got %d, want 5", got)
	}

	if got := s.AddCounter("PollCount", 7); got != 12 {
		t.Errorf("got %d, want 12", got)
	}

	got, ok := s.Counter("PollCount")
	if !ok || got != 12 {
		t.Errorf("got (%v, %v), want (12, true)", got, ok)
	}
}

func TestMemStorageAll(t *testing.T) {
	s := NewMemStorage()
	s.SetGauge("Alloc", 1)
	s.AddCounter("PollCount", 3)

	all := s.All()
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

var _ Storage = (*MemStorage)(nil)
