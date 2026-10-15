package postgres

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewRejectsInvalidDSN(t *testing.T) {
	db, err := New(context.Background(), "postgres://user:pass@localhost:notaport/db")
	if err == nil {
		db.Close()
		t.Fatal("invalid DSN must produce an error")
	}
}

func TestPingUnreachableDatabase(t *testing.T) {
	db, err := New(context.Background(), "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err == nil {
		t.Error("ping of an unreachable database must fail")
	}
}

func TestPingRealDatabase(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}

	db, err := New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}
