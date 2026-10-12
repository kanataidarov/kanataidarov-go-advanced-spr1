package repository

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type PersistentStorage struct {
	*MemStorage

	file     *FileStore
	syncMode bool
	log      *zap.Logger

	saveMu sync.Mutex
}

func NewPersistentStorage(mem *MemStorage, file *FileStore, syncMode bool, log *zap.Logger) *PersistentStorage {
	return &PersistentStorage{
		MemStorage: mem,
		file:       file,
		syncMode:   syncMode,
		log:        log,
	}
}

func (p *PersistentStorage) SetGauge(name string, value float64) {
	p.MemStorage.SetGauge(name, value)
	p.saveIfSync()
}

func (p *PersistentStorage) AddCounter(name string, delta int64) int64 {
	total := p.MemStorage.AddCounter(name, delta)
	p.saveIfSync()

	return total
}

func (p *PersistentStorage) Save() error {

	p.saveMu.Lock()
	defer p.saveMu.Unlock()

	return p.file.Save(p.All())
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
