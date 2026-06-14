package app

import (
	"context"
	"fmt"
	"time"
)

const sqliteSnapshotInterval = 6 * time.Hour

func (a *App) closeStorage() error {
	if a == nil || a.platform.Storage == nil {
		return nil
	}

	if err := a.platform.Storage.Close(); err != nil {
		return fmt.Errorf("close sqlite store: %w", err)
	}

	a.platform.Storage = nil
	return nil
}

func (a *App) startSQLiteSnapshotLoop(ctx context.Context) {
	if a == nil || a.platform.Storage == nil {
		return
	}

	go func() {
		a.createSQLiteSnapshotBestEffort(ctx)
		ticker := time.NewTicker(sqliteSnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.createSQLiteSnapshotBestEffort(ctx)
			}
		}
	}()
}

func (a *App) createSQLiteSnapshotBestEffort(parent context.Context) {
	if a == nil || a.platform.Storage == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()

	path, err := a.platform.Storage.CreateSnapshot(ctx)
	if err != nil {
		if a.state != nil && a.state.Logger != nil {
			a.state.Logger.Warn("sqlite snapshot failed", "component", "storage", "err", err.Error())
		}
		return
	}
	if a.state != nil && a.state.Logger != nil {
		a.state.Logger.Info("sqlite snapshot created", "component", "storage", "path", path)
	}
}
