package postgres

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

var _ service.Storage = (*DB)(nil)

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

	if err := db.Migrate(ctx); err == nil {
		t.Error("migrations on an unreachable database must fail")
	}
}

func newTestDB(t *testing.T) *DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	t.Cleanup(db.Close)

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if _, err := db.pool.Exec(ctx, "TRUNCATE gauges, counters"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return db
}

func TestPingRealDatabase(t *testing.T) {
	db := newTestDB(t)

	if err := db.Ping(t.Context()); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestStorageGauge(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	if _, ok, err := db.Gauge(ctx, "Alloc"); err != nil || ok {
		t.Fatalf("missing gauge: got (ok=%v, err=%v), want (false, nil)", ok, err)
	}

	for _, v := range []float64{1.5, 1257894000000000000.25} {
		if err := db.SetGauge(ctx, "Alloc", v); err != nil {
			t.Fatalf("set gauge: %v", err)
		}
	}

	if v, ok, err := db.Gauge(ctx, "Alloc"); err != nil || !ok || v != 1257894000000000000.25 {
		t.Errorf("got (%v, %v, %v), want the last value", v, ok, err)
	}
}

func TestStorageCounter(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	if _, ok, err := db.Counter(ctx, "PollCount"); err != nil || ok {
		t.Fatalf("missing counter: got (ok=%v, err=%v), want (false, nil)", ok, err)
	}

	for _, step := range []struct{ delta, want int64 }{{delta: 5, want: 5}, {delta: 7, want: 12}} {
		total, err := db.AddCounter(ctx, "PollCount", step.delta)
		if err != nil || total != step.want {
			t.Fatalf("add counter %d: got (%d, %v), want %d", step.delta, total, err, step.want)
		}
	}

	if v, ok, err := db.Counter(ctx, "PollCount"); err != nil || !ok || v != 12 {
		t.Errorf("got (%v, %v, %v), want (12, true, nil)", v, ok, err)
	}
}

func TestStorageCounterConcurrentAdds(t *testing.T) {
	db := newTestDB(t)

	var wg sync.WaitGroup

	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if _, err := db.AddCounter(context.Background(), "total", 1); err != nil {
				t.Errorf("add counter: %v", err)
			}
		}()
	}

	wg.Wait()

	if v, _, err := db.Counter(t.Context(), "total"); err != nil || v != 20 {
		t.Errorf("got (%d, %v), want 20: concurrent adds must not be lost", v, err)
	}
}

func TestStorageAll(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	if all, err := db.All(ctx); err != nil || len(all) != 0 {
		t.Fatalf("empty storage: got (%v, %v)", all, err)
	}

	for i := range 3 {
		if err := db.SetGauge(ctx, fmt.Sprintf("g%d", i), float64(i)); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.SetGauge(ctx, "Same", 0.5); err != nil {
		t.Fatal(err)
	}

	if _, err := db.AddCounter(ctx, "Same", 3); err != nil {
		t.Fatal(err)
	}

	all, err := db.All(ctx)
	if err != nil {
		t.Fatalf("all: %v", err)
	}

	if len(all) != 5 {
		t.Fatalf("got %d metrics, want 5: %+v", len(all), all)
	}

	for _, m := range all {
		switch m.MType {
		case models.Gauge:
			if m.Value == nil || m.Delta != nil {
				t.Errorf("malformed gauge %+v", m)
			}
		case models.Counter:
			if m.Delta == nil || m.Value != nil || m.ID != "Same" || *m.Delta != 3 {
				t.Errorf("malformed counter %+v", m)
			}
		default:
			t.Errorf("unexpected type in %+v", m)
		}
	}
}

func TestStorageUpdateMetrics(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	if _, err := db.AddCounter(ctx, "PollCount", 10); err != nil {
		t.Fatal(err)
	}

	err := db.UpdateMetrics(ctx, []models.Metrics{
		{ID: "PollCount", MType: models.Counter, Delta: ptrInt(5)},
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1.5)},
	})
	if err != nil {
		t.Fatalf("update metrics: %v", err)
	}

	if v, ok, err := db.Counter(ctx, "PollCount"); err != nil || !ok || v != 15 {
		t.Errorf("got counter (%v, %v, %v), want (15, true, nil)", v, ok, err)
	}

	if v, ok, err := db.Gauge(ctx, "Alloc"); err != nil || !ok || v != 1.5 {
		t.Errorf("got gauge (%v, %v, %v), want (1.5, true, nil)", v, ok, err)
	}
}

func TestStorageUpdateMetricsRollsBackOnError(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	if _, err := db.AddCounter(ctx, "Big", math.MaxInt64); err != nil {
		t.Fatal(err)
	}

	err := db.UpdateMetrics(ctx, []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1)},
		{ID: "Big", MType: models.Counter, Delta: ptrInt(1)},
	})
	if err == nil {
		t.Fatal("expected an overflow error from the database")
	}

	if _, ok, err := db.Gauge(ctx, "Alloc"); err != nil || ok {
		t.Errorf("got (ok=%v, err=%v): a failed batch must not be saved partially", ok, err)
	}

	if v, _, err := db.Counter(ctx, "Big"); err != nil || v != math.MaxInt64 {
		t.Errorf("got (%d, %v): counter must keep its value after rollback", v, err)
	}
}

func TestStorageUpdateMetricsConcurrentBatches(t *testing.T) {
	db := newTestDB(t)

	batch := []models.Metrics{
		{ID: "A", MType: models.Counter, Delta: ptrInt(1)},
		{ID: "B", MType: models.Counter, Delta: ptrInt(2)},
		{ID: "C", MType: models.Gauge, Value: ptrFloat(3)},
	}

	var wg sync.WaitGroup

	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := db.UpdateMetrics(context.Background(), batch); err != nil {
				t.Errorf("update metrics: %v", err)
			}
		}()
	}

	wg.Wait()

	for name, want := range map[string]int64{"A": 20, "B": 40} {
		if v, _, err := db.Counter(t.Context(), name); err != nil || v != want {
			t.Errorf("counter %s: got (%d, %v), want %d", name, v, err, want)
		}
	}
}

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }
