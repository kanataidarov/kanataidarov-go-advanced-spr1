package repository

import (
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

func (s *MemStorage) SetGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges[name] = value
}

func (s *MemStorage) AddCounter(name string, delta int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters[name] += delta

	return s.counters[name]
}

func (s *MemStorage) Gauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.gauges[name]

	return v, ok
}

func (s *MemStorage) Counter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.counters[name]

	return v, ok
}

func (s *MemStorage) All() []models.Metrics {
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

func (s *MemStorage) Restore(metrics []models.Metrics) error {
	gauges := make(map[string]float64, len(metrics))
	counters := make(map[string]int64, len(metrics))

	for _, m := range metrics {
		switch {
		case m.ID == "":
			return fmt.Errorf("metric of type %q without name", m.MType)
		case m.MType == models.Gauge && m.Value != nil:
			gauges[m.ID] = *m.Value
		case m.MType == models.Counter && m.Delta != nil:
			counters[m.ID] = *m.Delta
		default:
			return fmt.Errorf("invalid metric %q of type %q", m.ID, m.MType)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges = gauges
	s.counters = counters

	return nil
}
