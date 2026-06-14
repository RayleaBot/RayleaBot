package storage

import (
	"context"
	"log/slog"
	"time"
)

const SnapshotInterval = 6 * time.Hour

func StartSnapshotLoop(ctx context.Context, store *Store, logger *slog.Logger) {
	if store == nil {
		return
	}

	go func() {
		CreateSnapshotBestEffort(ctx, store, logger)
		ticker := time.NewTicker(SnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				CreateSnapshotBestEffort(ctx, store, logger)
			}
		}
	}()
}

func CreateSnapshotBestEffort(parent context.Context, store *Store, logger *slog.Logger) {
	if store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()

	path, err := store.CreateSnapshot(ctx)
	if err != nil {
		if logger != nil {
			logger.Warn("sqlite snapshot failed", "component", "storage", "err", err.Error())
		}
		return
	}
	if logger != nil {
		logger.Info("sqlite snapshot created", "component", "storage", "path", path)
	}
}
