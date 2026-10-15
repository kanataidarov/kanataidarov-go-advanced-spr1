package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/migrations"
)

type DB struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot create postgres connection pool: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Migrate(ctx context.Context) error {
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
