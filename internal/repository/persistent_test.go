package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

func loadMap(t *testing.T, store *FileStore) map[string]models.Metrics {
	t.Helper()

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	out := make(map[string]models.Metrics, len(loaded))
	for _, m := range loaded {
		out[m.ID] = m
	}

	return out
}

func TestPersistentStorageSyncMode(t *testing.T) {
	file := NewFileStore(filepath.Join(t.TempDir(), "metrics.json"))
	p := NewPersistentStorage(NewMemStorage(), file, true, zap.NewNop())

	if err := p.SetGauge(t.Context(), "Alloc", 12.5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := loadMap(t, file)["Alloc"]; got.Value == nil || *got.Value != 12.5 {
		t.Fatalf("gauge was not saved synchronously: %+v", got)
	}

	if total, err := p.AddCounter(t.Context(), "PollCount", 3); err != nil || total != 3 {
		t.Errorf("got (%d, %v), want (3, nil)", total, err)
	}

	if _, err := p.AddCounter(t.Context(), "PollCount", 4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := loadMap(t, file)["PollCount"]; got.Delta == nil || *got.Delta != 7 {
		t.Errorf("counter was not saved synchronously: %+v", got)
	}
}

func TestPersistentStorageSyncModeConcurrentUpdates(t *testing.T) {
	file := NewFileStore(filepath.Join(t.TempDir(), "metrics.json"))
	p := NewPersistentStorage(NewMemStorage(), file, true, zap.NewNop())

	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()
			_ = p.SetGauge(context.Background(), fmt.Sprintf("g%d", i), float64(i))
			_, _ = p.AddCounter(context.Background(), "total", 1)
		}()
	}

	wg.Wait()

	saved := loadMap(t, file)
	if len(saved) != 21 {
		t.Errorf("got %d saved metrics, want 21: the last save must contain every update", len(saved))
	}

	if got := saved["total"]; got.Delta == nil || *got.Delta != 20 {
		t.Errorf("got total %+v, want 20", got)
	}
}

func TestPersistentStoragePeriodicMode(t *testing.T) {
	file := NewFileStore(filepath.Join(t.TempDir(), "metrics.json"))
	p := NewPersistentStorage(NewMemStorage(), file, false, zap.NewNop())

	if err := p.SetGauge(t.Context(), "Alloc", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if saved := loadMap(t, file); len(saved) != 0 {
		t.Fatalf("periodic mode must not save on update, got %+v", saved)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		p.Run(ctx, 10*time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for len(loadMap(t, file)) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}

	if got := loadMap(t, file)["Alloc"]; got.Value == nil || *got.Value != 1 {
		t.Errorf("metric was not saved by Run: %+v", got)
	}
}

func TestPersistentStorageLogsSaveErrors(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	file := NewFileStore(filepath.Join(t.TempDir(), "no-such-dir", "metrics.json"))
	p := NewPersistentStorage(NewMemStorage(), file, true, zap.New(core))

	if err := p.SetGauge(t.Context(), "Alloc", 1); err != nil {
		t.Fatalf("a failed file save must not fail the update: %v", err)
	}

	if v, ok, _ := p.Gauge(t.Context(), "Alloc"); !ok || v != 1 {
		t.Errorf("value must stay in memory even if saving fails, got (%v, %v)", v, ok)
	}

	entries := logs.All()
	if len(entries) != 1 || entries[0].Level != zap.ErrorLevel {
		t.Fatalf("got log entries %+v, want one error", entries)
	}
}

func TestPersistentStorageSyncModeSavesBatch(t *testing.T) {
	file := NewFileStore(filepath.Join(t.TempDir(), "metrics.json"))
	p := NewPersistentStorage(NewMemStorage(), file, true, zap.NewNop())

	err := p.UpdateMetrics(t.Context(), []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(1.5)},
		{ID: "PollCount", MType: models.Counter, Delta: ptrInt(3)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	saved := loadMap(t, file)
	if got := saved["Alloc"]; got.Value == nil || *got.Value != 1.5 {
		t.Errorf("gauge was not saved: %+v", got)
	}

	if got := saved["PollCount"]; got.Delta == nil || *got.Delta != 3 {
		t.Errorf("counter was not saved: %+v", got)
	}
}

func TestPersistentStoragePingIsAlwaysHealthy(t *testing.T) {
	file := NewFileStore(filepath.Join(t.TempDir(), "metrics.json"))
	p := NewPersistentStorage(NewMemStorage(), file, true, zap.NewNop())

	if err := p.Ping(t.Context()); err != nil {
		t.Errorf("file-backed storage is always available, got %v", err)
	}
}
