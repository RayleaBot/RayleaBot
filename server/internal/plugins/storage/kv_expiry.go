package storage

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const (
	KVSweepInterval = 60 * time.Second
)

// SweepExpired releases the single writer after each batch. Logical reads and
// quota accounting do not depend on physical cleanup finishing.
func (r *KVSQLiteRepository) SweepExpired(ctx context.Context) (int64, error) {
	var count int64
	err := storage.WithTx(ctx, r.write, nil, func(tx *sql.Tx) error {
		var err error
		count, err = r.writeQ.WithTx(tx).DeleteExpiredKV(ctx, sql.NullInt64{Int64: r.now().UnixMilli(), Valid: true})
		return err
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// The App supervisor owns this loop and waits for it before closing SQLite.
func RunKVExpiryLoop(ctx context.Context, repo KVRepository, logger *slog.Logger) {
	ticker := time.NewTicker(KVSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := repo.SweepExpired(ctx); err != nil && ctx.Err() == nil {
				logger.Warn("过期 KV 清理失败，将在下一轮重试", "component", "storage", "err", err)
			}
		}
	}
}
