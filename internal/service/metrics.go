package service

import (
	"context"
	"errors"
	"fmt"
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
	SetGauge(ctx context.Context, name string, value float64) error
	AddCounter(ctx context.Context, name string, delta int64) (int64, error)
	Gauge(ctx context.Context, name string) (float64, bool, error)
	Counter(ctx context.Context, name string) (int64, bool, error)
	All(ctx context.Context) ([]models.Metrics, error)
}

type MetricsService struct {
	storage Storage
}

func NewMetricsService(storage Storage) *MetricsService {
	return &MetricsService{storage: storage}
}

func (s *MetricsService) Update(ctx context.Context, mType, name, rawValue string) error {
	if name == "" {
		return ErrEmptyName
	}

	switch mType {
	case models.Gauge:
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return ErrInvalidValue
		}

		return s.setGauge(ctx, name, value)
	case models.Counter:
		delta, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return ErrInvalidValue
		}

		_, err = s.addCounter(ctx, name, delta)

		return err
	default:
		return ErrUnknownType
	}
}

func (s *MetricsService) Value(ctx context.Context, mType, name string) (string, error) {
	m, err := s.Metric(ctx, mType, name)
	if err != nil {
		return "", err
	}

	if m.MType == models.Gauge {
		return FormatGauge(*m.Value), nil
	}

	return FormatCounter(*m.Delta), nil
}

func (s *MetricsService) UpdateMetric(ctx context.Context, m models.Metrics) (models.Metrics, error) {
	if m.ID == "" {
		return models.Metrics{}, ErrEmptyName
	}

	switch m.MType {
	case models.Gauge:
		if m.Value == nil {
			return models.Metrics{}, ErrInvalidValue
		}

		value := *m.Value
		if err := s.setGauge(ctx, m.ID, value); err != nil {
			return models.Metrics{}, err
		}

		return models.Metrics{ID: m.ID, MType: models.Gauge, Value: &value}, nil
	case models.Counter:
		if m.Delta == nil {
			return models.Metrics{}, ErrInvalidValue
		}

		total, err := s.addCounter(ctx, m.ID, *m.Delta)
		if err != nil {
			return models.Metrics{}, err
		}

		return models.Metrics{ID: m.ID, MType: models.Counter, Delta: &total}, nil
	default:
		return models.Metrics{}, ErrUnknownType
	}
}

func (s *MetricsService) Metric(ctx context.Context, mType, name string) (models.Metrics, error) {
	if name == "" {
		return models.Metrics{}, ErrEmptyName
	}

	switch mType {
	case models.Gauge:
		value, ok, err := s.storage.Gauge(ctx, name)
		if err != nil {
			return models.Metrics{}, fmt.Errorf("cannot read gauge %q: %w", name, err)
		}

		if !ok {
			return models.Metrics{}, ErrNotFound
		}

		return models.Metrics{ID: name, MType: models.Gauge, Value: &value}, nil
	case models.Counter:
		delta, ok, err := s.storage.Counter(ctx, name)
		if err != nil {
			return models.Metrics{}, fmt.Errorf("cannot read counter %q: %w", name, err)
		}

		if !ok {
			return models.Metrics{}, ErrNotFound
		}

		return models.Metrics{ID: name, MType: models.Counter, Delta: &delta}, nil
	default:
		return models.Metrics{}, ErrUnknownType
	}
}

func (s *MetricsService) All(ctx context.Context) ([]models.Metrics, error) {
	all, err := s.storage.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read metrics: %w", err)
	}

	return all, nil
}

func (s *MetricsService) setGauge(ctx context.Context, name string, value float64) error {
	if err := s.storage.SetGauge(ctx, name, value); err != nil {
		return fmt.Errorf("cannot save gauge %q: %w", name, err)
	}

	return nil
}

func (s *MetricsService) addCounter(ctx context.Context, name string, delta int64) (int64, error) {
	total, err := s.storage.AddCounter(ctx, name, delta)
	if err != nil {
		return 0, fmt.Errorf("cannot save counter %q: %w", name, err)
	}

	return total, nil
}

func FormatGauge(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func FormatCounter(delta int64) string {
	return strconv.FormatInt(delta, 10)
}
