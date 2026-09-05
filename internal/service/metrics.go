package service

import (
	"errors"
	"strconv"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
)

var (
	ErrEmptyName    = errors.New("metric name is not specified")
	ErrUnknownType  = errors.New("unknown metric type")
	ErrInvalidValue = errors.New("invalid metric value")
)

type MetricsService struct {
	storage repository.Storage
}

func NewMetricsService(storage repository.Storage) *MetricsService {
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
			return "", ErrEmptyName
		}

		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case models.Counter:
		delta, ok := s.storage.Counter(name)
		if !ok {
			return "", ErrEmptyName
		}

		return strconv.FormatInt(delta, 10), nil
	default:
		return "", ErrUnknownType
	}
}

func (s *MetricsService) All() []models.Metrics {
	return s.storage.All()
}
