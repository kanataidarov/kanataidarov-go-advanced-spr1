package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/retry"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/migrations"
)

const migrateTimeout = 10 * time.Second

type DB struct {
	pool  *pgxpool.Pool
	retry retry.Policy
}

func New(ctx context.Context, dsn string, log *zap.Logger) (*DB, error) {
	db, err := open(ctx, dsn, log)
	if err != nil {
		return nil, err
	}

	migrateCtx, cancel := context.WithTimeout(ctx, migrateTimeout)
	defer cancel()

	if err := db.migrate(migrateCtx); err != nil {
		db.Close()

		return nil, err
	}

	return db, nil
}

func open(ctx context.Context, dsn string, log *zap.Logger) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot create postgres connection pool: %w", err)
	}

	onRetry := func(err error, attempt int, delay time.Duration) {
		log.Warn("retriable database error, retrying",
			zap.Int("attempt", attempt), zap.Duration("delay", delay), zap.Error(err))
	}

	return &DB{pool: pool, retry: retry.New(isRetriable, onRetry)}, nil
}

func (db *DB) migrate(ctx context.Context) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(db.pool), migrations.FS)
	if err != nil {
		return fmt.Errorf("cannot create migration provider: %w", err)
	}
	defer provider.Close()

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("cannot apply migrations: %w", err)
	}

	return nil
}

func (db *DB) Ping(ctx context.Context) error {
	if err := db.pool.Ping(ctx); err != nil {
		return fmt.Errorf("cannot ping postgres: %w", err)
	}

	return nil
}

func (db *DB) Close() {
	db.pool.Close()
}
