package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

const (
	upsertGauge = `
		INSERT INTO gauges (name, value) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value`

	upsertCounter = `
		INSERT INTO counters (name, delta) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET delta = counters.delta + EXCLUDED.delta
		RETURNING delta`

	selectGauge   = `SELECT value FROM gauges WHERE name = $1`
	selectCounter = `SELECT delta FROM counters WHERE name = $1`

	selectAll = `
		SELECT name, 'gauge' AS type, value, NULL::BIGINT AS delta FROM gauges
		UNION ALL
		SELECT name, 'counter' AS type, NULL::DOUBLE PRECISION AS value, delta FROM counters`
)

func (db *DB) SetGauge(ctx context.Context, name string, value float64) error {
	err := db.retry.Do(ctx, func(ctx context.Context) error {
		_, err := db.pool.Exec(ctx, upsertGauge, name, value)

		return err
	})
	if err != nil {
		return fmt.Errorf("upsert gauge: %w", err)
	}

	return nil
}

func (db *DB) AddCounter(ctx context.Context, name string, delta int64) (int64, error) {
	var total int64

	err := db.retry.Do(ctx, func(ctx context.Context) error {
		return db.pool.QueryRow(ctx, upsertCounter, name, delta).Scan(&total)
	})
	if err != nil {
		return 0, fmt.Errorf("upsert counter: %w", err)
	}

	return total, nil
}

func (db *DB) Gauge(ctx context.Context, name string) (float64, bool, error) {
	var value float64

	err := db.retry.Do(ctx, func(ctx context.Context) error {
		return db.pool.QueryRow(ctx, selectGauge, name).Scan(&value)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}

	if err != nil {
		return 0, false, fmt.Errorf("select gauge: %w", err)
	}

	return value, true, nil
}

func (db *DB) Counter(ctx context.Context, name string) (int64, bool, error) {
	var delta int64

	err := db.retry.Do(ctx, func(ctx context.Context) error {
		return db.pool.QueryRow(ctx, selectCounter, name).Scan(&delta)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}

	if err != nil {
		return 0, false, fmt.Errorf("select counter: %w", err)
	}

	return delta, true, nil
}

func (db *DB) All(ctx context.Context) ([]models.Metrics, error) {
	var all []models.Metrics

	err := db.retry.Do(ctx, func(ctx context.Context) error {
		rows, err := db.pool.Query(ctx, selectAll)
		if err != nil {
			return err
		}

		all, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.Metrics, error) {
			var m models.Metrics
			err := row.Scan(&m.ID, &m.MType, &m.Value, &m.Delta)

			return m, err
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("select metrics: %w", err)
	}

	return all, nil
}

func (db *DB) UpdateMetrics(ctx context.Context, metrics []models.Metrics) error {
	for _, m := range metrics {
		if !(m.MType == models.Gauge && m.Value != nil) && !(m.MType == models.Counter && m.Delta != nil) {
			return fmt.Errorf("invalid metric %q of type %q", m.ID, m.MType)
		}
	}

	if err := db.retry.Do(ctx, func(ctx context.Context) error {
		return db.applyBatch(ctx, metrics)
	}); err != nil {
		return fmt.Errorf("update metrics: %w", err)
	}

	return nil
}

func (db *DB) applyBatch(ctx context.Context, metrics []models.Metrics) error {
	batch := &pgx.Batch{}

	for _, m := range metrics {
		if m.MType == models.Gauge {
			batch.Queue(upsertGauge, m.ID, *m.Value)
		} else {
			batch.Queue(upsertCounter, m.ID, *m.Delta)
		}
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
