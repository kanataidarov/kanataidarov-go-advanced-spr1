package service

import (
	"errors"
	"strconv"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

var (
	ErrEmptyName    = errors.New("metric name is not specified")
	ErrUnknownType  = errors.New("unknown metric type")
	ErrInvalidValue = errors.New("invalid metric value")
	ErrNotFound     = errors.New("metric not found")
)

type Storage interface {
	SetGauge(name string, value float64)
	AddCounter(name string, delta int64) int64
	Gauge(name string) (float64, bool)
	Counter(name string) (int64, bool)
	All() []models.Metrics
}

type MetricsService struct {
	storage Storage
}

func NewMetricsService(storage Storage) *MetricsService {
	return &MetricsService{storage: storage}
}

func (s *MetricsService) Update(mType, name, rawValue string) error {
	if name == "" {
		return ErrEmptyName
	}

	switch mType {
	case models.Gauge:
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return ErrInvalidValue
		}

		s.storage.SetGauge(name, value)

		return nil
	case models.Counter:
		delta, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return ErrInvalidValue
		}

		s.storage.AddCounter(name, delta)

		return nil
	default:
		return ErrUnknownType
	}
}

func (s *MetricsService) Value(mType, name string) (string, error) {
	if name == "" {
		return "", ErrEmptyName
	}

	switch mType {
	case models.Gauge:
		value, ok := s.storage.Gauge(name)
		if !ok {
			return "", ErrNotFound
		}

		return FormatGauge(value), nil
	case models.Counter:
		delta, ok := s.storage.Counter(name)
		if !ok {
			return "", ErrNotFound
		}

		return FormatCounter(delta), nil
	default:
		return "", ErrUnknownType
	}
}

func (s *MetricsService) UpdateMetric(m models.Metrics) (models.Metrics, error) {
	if m.ID == "" {
		return models.Metrics{}, ErrEmptyName
	}

	switch m.MType {
	case models.Gauge:
		if m.Value == nil {
			return models.Metrics{}, ErrInvalidValue
		}

		value := *m.Value
		s.storage.SetGauge(m.ID, value)

		return models.Metrics{ID: m.ID, MType: models.Gauge, Value: &value}, nil
	case models.Counter:
		if m.Delta == nil {
			return models.Metrics{}, ErrInvalidValue
		}

		total := s.storage.AddCounter(m.ID, *m.Delta)

		return models.Metrics{ID: m.ID, MType: models.Counter, Delta: &total}, nil
	default:
		return models.Metrics{}, ErrUnknownType
	}
}

func (s *MetricsService) Metric(mType, name string) (models.Metrics, error) {
	if name == "" {
		return models.Metrics{}, ErrEmptyName
	}

	switch mType {
	case models.Gauge:
		value, ok := s.storage.Gauge(name)
		if !ok {
			return models.Metrics{}, ErrNotFound
		}

		return models.Metrics{ID: name, MType: models.Gauge, Value: &value}, nil
	case models.Counter:
		delta, ok := s.storage.Counter(name)
		if !ok {
			return models.Metrics{}, ErrNotFound
		}

		return models.Metrics{ID: name, MType: models.Counter, Delta: &delta}, nil
	default:
		return models.Metrics{}, ErrUnknownType
	}
}

func (s *MetricsService) All() []models.Metrics {
	return s.storage.All()
}

func FormatGauge(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func FormatCounter(delta int64) string {
	return strconv.FormatInt(delta, 10)
}
