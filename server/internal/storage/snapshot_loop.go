package storage

import (
	"context"
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
)

const SnapshotInterval = 6 * time.Hour

func StartSnapshotLoop(ctx context.Context, store *Store, logger *slog.Logger, repoRoot string) {
	if store == nil {
		return
	}

	go func() {
		CreateSnapshotBestEffort(ctx, store, logger, repoRoot)
		ticker := time.NewTicker(SnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				CreateSnapshotBestEffort(ctx, store, logger, repoRoot)
			}
		}
	}()
}

func CreateSnapshotBestEffort(parent context.Context, store *Store, logger *slog.Logger, repoRoot string) {
	if store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()

	path, err := store.CreateSnapshot(ctx)
	if err != nil {
		if logger != nil {
			logger.Warn("SQLite 数据库快照创建失败", "component", "storage", "err", logpath.Error(repoRoot, err, store.Path, SnapshotDirForDatabase(store.Path)))
		}
		return
	}
	if logger != nil {
		pathDisplay := logpath.Display(repoRoot, path)
		logger.Info("SQLite 数据库快照已创建："+pathDisplay, "component", "storage", "path", pathDisplay)
	}
}
