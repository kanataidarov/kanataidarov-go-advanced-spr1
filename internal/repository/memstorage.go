package repository

import (
	"context"
	"fmt"
	"sync"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) SetGauge(_ context.Context, name string, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges[name] = value

	return nil
}

func (s *MemStorage) AddCounter(_ context.Context, name string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters[name] += delta

	return s.counters[name], nil
}

func (s *MemStorage) Gauge(_ context.Context, name string) (float64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.gauges[name]

	return v, ok, nil
}

func (s *MemStorage) Counter(_ context.Context, name string) (int64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.counters[name]

	return v, ok, nil
}

func (s *MemStorage) Ping(_ context.Context) error {
	return nil
}

func (s *MemStorage) All(_ context.Context) ([]models.Metrics, error) {
	return s.snapshot(), nil
}

func (s *MemStorage) snapshot() []models.Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]models.Metrics, 0, len(s.gauges)+len(s.counters))

	for name, value := range s.gauges {
		v := value
		all = append(all, models.Metrics{ID: name, MType: models.Gauge, Value: &v})
	}

	for name, delta := range s.counters {
		d := delta
		all = append(all, models.Metrics{ID: name, MType: models.Counter, Delta: &d})
	}

	return all
}

func (s *MemStorage) UpdateMetrics(_ context.Context, metrics []models.Metrics) error {
	for _, m := range metrics {
		if err := checkMetric(m); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metrics {
		if m.MType == models.Gauge {
			s.gauges[m.ID] = *m.Value
		} else {
			s.counters[m.ID] += *m.Delta
		}
	}

	return nil
}

func (s *MemStorage) Restore(metrics []models.Metrics) error {
	gauges := make(map[string]float64, len(metrics))
	counters := make(map[string]int64, len(metrics))

	for _, m := range metrics {
		if err := checkMetric(m); err != nil {
			return err
		}

		if m.MType == models.Gauge {
			gauges[m.ID] = *m.Value
		} else {
			counters[m.ID] = *m.Delta
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges = gauges
	s.counters = counters

	return nil
}

func checkMetric(m models.Metrics) error {
	switch {
	case m.ID == "":
		return fmt.Errorf("metric of type %q without name", m.MType)
	case m.MType == models.Gauge && m.Value != nil, m.MType == models.Counter && m.Delta != nil:
		return nil
	default:
		return fmt.Errorf("invalid metric %q of type %q", m.ID, m.MType)
	}
}
