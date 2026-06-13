package pluginkv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) Set(ctx context.Context, pluginID, key string, value any, limits Limits) error {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)

	valueJSON, sizeBytes, err := encodeValue(key, value)
	if err != nil {
		return err
	}
	if limits.ValueMaxBytes > 0 && len(valueJSON) > limits.ValueMaxBytes {
		return ErrValueTooLarge
	}

	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin plugin kv transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	q := r.writeQ.WithTx(tx)

	previousSize, err := q.GetKVSize(ctx, sqlcgen.GetKVSizeParams{
		PluginID: pluginID,
		Key:      key,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query previous plugin kv size: %w", err)
	}

	totalSize, err := q.GetKVTotalSize(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("query plugin kv total size: %w", err)
	}

	nextTotal := int(totalSize) - int(previousSize) + sizeBytes
	if limits.TotalMaxBytes > 0 && nextTotal > limits.TotalMaxBytes {
		return ErrQuotaExceeded
	}

	if err := q.UpsertKV(ctx, sqlcgen.UpsertKVParams{
		PluginID:  pluginID,
		Key:       key,
		ValueJson: string(valueJSON),
		SizeBytes: int64(sizeBytes),
		UpdatedAt: r.now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("upsert plugin kv value: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit plugin kv transaction: %w", err)
	}
	return nil
}
