package repository

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

type PersistentStorage struct {
	mem *MemStorage

	file     *FileStore
	syncMode bool
	log      *zap.Logger

	saveMu sync.Mutex
}

func NewPersistentStorage(mem *MemStorage, file *FileStore, syncMode bool, log *zap.Logger) *PersistentStorage {
	return &PersistentStorage{
		mem:      mem,
		file:     file,
		syncMode: syncMode,
		log:      log,
	}
}

func (p *PersistentStorage) SetGauge(ctx context.Context, name string, value float64) error {
	if err := p.mem.SetGauge(ctx, name, value); err != nil {
		return err
	}

	p.saveIfSync()

	return nil
}

func (p *PersistentStorage) AddCounter(ctx context.Context, name string, delta int64) (int64, error) {
	total, err := p.mem.AddCounter(ctx, name, delta)
	if err != nil {
		return 0, err
	}

	p.saveIfSync()

	return total, nil
}

func (p *PersistentStorage) UpdateMetrics(ctx context.Context, metrics []models.Metrics) error {
	if err := p.mem.UpdateMetrics(ctx, metrics); err != nil {
		return err
	}

	p.saveIfSync()

	return nil
}

func (p *PersistentStorage) Gauge(ctx context.Context, name string) (float64, bool, error) {
	return p.mem.Gauge(ctx, name)
}

func (p *PersistentStorage) Counter(ctx context.Context, name string) (int64, bool, error) {
	return p.mem.Counter(ctx, name)
}

func (p *PersistentStorage) All(ctx context.Context) ([]models.Metrics, error) {
	return p.mem.All(ctx)
}

func (p *PersistentStorage) Ping(_ context.Context) error {
	return nil
}

func (p *PersistentStorage) Save() error {
	p.saveMu.Lock()
	defer p.saveMu.Unlock()

	return p.file.Save(p.mem.snapshot())
}

func (p *PersistentStorage) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.saveLogged()
		}
	}
}

func (p *PersistentStorage) saveIfSync() {
	if p.syncMode {
		p.saveLogged()
	}
}

func (p *PersistentStorage) saveLogged() {
	if err := p.Save(); err != nil {
		p.log.Error("cannot save metrics", zap.String("path", p.file.Path()), zap.Error(err))
	}
}
