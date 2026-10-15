package repository

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

// PersistentStorage wraps MemStorage without embedding it: every method is
// delegated explicitly, so a new storage method cannot silently bypass saving.
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

func (p *PersistentStorage) SetGauge(name string, value float64) {
	p.mem.SetGauge(name, value)
	p.saveIfSync()
}

func (p *PersistentStorage) AddCounter(name string, delta int64) int64 {
	total := p.mem.AddCounter(name, delta)
	p.saveIfSync()

	return total
}

func (p *PersistentStorage) Gauge(name string) (float64, bool) {
	return p.mem.Gauge(name)
}

func (p *PersistentStorage) Counter(name string) (int64, bool) {
	return p.mem.Counter(name)
}

func (p *PersistentStorage) All() []models.Metrics {
	return p.mem.All()
}

func (p *PersistentStorage) Save() error {
	p.saveMu.Lock()
	defer p.saveMu.Unlock()

	return p.file.Save(p.mem.All())
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
