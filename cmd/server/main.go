package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/compress"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/config"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/handler"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/logger"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/repository/postgres"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

const shutdownTimeout = 5 * time.Second

type pingableStorage interface {
	service.Storage
	handler.Pinger
}

type unavailableDB struct {
	err error
}

func (u unavailableDB) Ping(_ context.Context) error {
	return u.err
}

func main() {
	zlog, err := logger.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot initialize logger: %v\n", err)
		os.Exit(1)
	}

	if err := run(zlog); err != nil {
		zlog.Fatal("server stopped", zap.Error(err))
	}

	_ = zlog.Sync()
}

func run(zlog *zap.Logger) error {
	cfg, err := config.NewServerConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mem := repository.NewMemStorage()

	var (
		storage     pingableStorage = mem
		storageKind                 = "memory"
		persistent  *repository.PersistentStorage
	)

	db, dbErr := openDatabase(ctx, cfg.DatabaseDSN, zlog)
	if db != nil {
		defer db.Close()

		storage, storageKind = db, "postgres"
	} else if cfg.FileStoragePath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.FileStoragePath), 0o750); err != nil {
			return fmt.Errorf("cannot create directory for %s: %w", cfg.FileStoragePath, err)
		}

		file := repository.NewFileStore(cfg.FileStoragePath)

		if cfg.Restore {
			if err := restore(mem, file, zlog); err != nil {
				return err
			}
		}

		persistent = repository.NewPersistentStorage(mem, file, cfg.StoreInterval == 0, zlog)
		storage, storageKind = persistent, "file"
	}

	var pinger handler.Pinger = storage
	if dbErr != nil {
		pinger = unavailableDB{err: dbErr}
	}

	metrics := service.NewMetricsService(storage)
	metricsHandler := handler.NewMetricsHandler(metrics, zlog, handler.WithPinger(pinger))

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           logger.RequestLogger(zlog)(compress.Gzip(metricsHandler.Router())),
		ReadHeaderTimeout: 5 * time.Second,
	}

	saverCtx, stopSaver := context.WithCancel(ctx)
	defer stopSaver()

	var saver sync.WaitGroup

	if persistent != nil && cfg.StoreInterval > 0 {
		saver.Add(1)

		go func() {
			defer saver.Done()
			persistent.Run(saverCtx, cfg.StoreInterval)
		}()
	}

	zlog.Info("metrics server is listening",
		zap.String("address", cfg.Address),
		zap.String("storage", storageKind),
		zap.String("file_storage_path", cfg.FileStoragePath),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.Bool("restore", cfg.Restore),
	)

	serveErr := make(chan error, 1)

	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		stopSaver()
		saver.Wait()

		return errors.Join(fmt.Errorf("http server: %w", err), saveOnExit(persistent, cfg.FileStoragePath, zlog))
	case <-ctx.Done():
		zlog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		zlog.Error("cannot stop http server gracefully", zap.Error(err))
	}

	stopSaver()
	saver.Wait()

	return saveOnExit(persistent, cfg.FileStoragePath, zlog)
}

// Порядок проверки хранения: PostgreSQL (DATABASE_DSN / -d), затем файл, затем память.
// Ошибка возвращается, только если DSN задан, но БД недоступна.
func openDatabase(ctx context.Context, dsn string, zlog *zap.Logger) (*postgres.DB, error) {
	if dsn == "" {
		return nil, nil
	}

	db, err := postgres.New(ctx, dsn, zlog)
	if err != nil {
		zlog.Error("cannot use database, falling back to file or memory storage", zap.Error(err))

		return nil, err
	}

	return db, nil
}

func saveOnExit(persistent *repository.PersistentStorage, path string, zlog *zap.Logger) error {
	if persistent == nil {
		return nil
	}

	if err := persistent.Save(); err != nil {
		return fmt.Errorf("cannot save metrics on shutdown: %w", err)
	}

	zlog.Info("metrics saved", zap.String("path", path))

	return nil
}

func restore(mem *repository.MemStorage, file *repository.FileStore, zlog *zap.Logger) error {
	saved, err := file.Load()
	if err != nil {
		return fmt.Errorf("cannot restore metrics: %w", err)
	}

	if err := mem.Restore(saved); err != nil {
		return fmt.Errorf("cannot restore metrics from %s: %w", file.Path(), err)
	}

	zlog.Info("metrics restored", zap.String("path", file.Path()), zap.Int("count", len(saved)))

	return nil
}
