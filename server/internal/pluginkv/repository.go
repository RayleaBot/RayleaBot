package pluginkv

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"rayleabot/server/internal/storage"
)

var (
	ErrValueTooLarge = errors.New("plugin kv value exceeds configured limit")
	ErrQuotaExceeded = errors.New("plugin kv total capacity exceeds configured limit")
)

type Limits struct {
	ValueMaxBytes int
	TotalMaxBytes int
}

type Repository interface {
	Get(context.Context, string, string) (any, bool, error)
	Set(context.Context, string, string, any, Limits) error
	Delete(context.Context, string, string) (bool, error)
	List(context.Context, string, string) ([]string, error)
}

type SQLiteRepository struct {
	read  *sql.DB
	write *sql.DB
	now   func() time.Time
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		read:  store.Read,
		write: store.Write,
		now:   time.Now,
	}, nil
}

func (r *SQLiteRepository) Get(ctx context.Context, pluginID, key string) (any, bool, error) {
	var valueJSON string
	err := r.read.QueryRowContext(
		ctx,
		`SELECT value_json FROM plugin_kv WHERE plugin_id = ? AND key = ?`,
		strings.TrimSpace(pluginID),
		strings.TrimSpace(key),
	).Scan(&valueJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("query plugin kv value: %w", err)
	}

	var value any
	if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
		return nil, false, fmt.Errorf("decode plugin kv value: %w", err)
	}
	return value, true, nil
}

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

	var previousSize int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COALESCE(size_bytes, 0) FROM plugin_kv WHERE plugin_id = ? AND key = ?`,
		pluginID,
		key,
	).Scan(&previousSize); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query previous plugin kv size: %w", err)
	}

	var totalSize int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COALESCE(SUM(size_bytes), 0) FROM plugin_kv WHERE plugin_id = ?`,
		pluginID,
	).Scan(&totalSize); err != nil {
		return fmt.Errorf("query plugin kv total size: %w", err)
	}

	nextTotal := totalSize - previousSize + sizeBytes
	if limits.TotalMaxBytes > 0 && nextTotal > limits.TotalMaxBytes {
		return ErrQuotaExceeded
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO plugin_kv (plugin_id, key, value_json, size_bytes, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(plugin_id, key) DO UPDATE SET
		 	value_json = excluded.value_json,
		 	size_bytes = excluded.size_bytes,
		 	updated_at = excluded.updated_at`,
		pluginID,
		key,
		string(valueJSON),
		sizeBytes,
		r.now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert plugin kv value: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit plugin kv transaction: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, pluginID, key string) (bool, error) {
	result, err := r.write.ExecContext(
		ctx,
		`DELETE FROM plugin_kv WHERE plugin_id = ? AND key = ?`,
		strings.TrimSpace(pluginID),
		strings.TrimSpace(key),
	)
	if err != nil {
		return false, fmt.Errorf("delete plugin kv value: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read plugin kv delete result: %w", err)
	}
	return rows > 0, nil
}

func (r *SQLiteRepository) List(ctx context.Context, pluginID, prefix string) ([]string, error) {
	rows, err := r.read.QueryContext(
		ctx,
		`SELECT key
		 FROM plugin_kv
		 WHERE plugin_id = ? AND key LIKE ? ESCAPE '\'
		 ORDER BY key ASC`,
		strings.TrimSpace(pluginID),
		escapeLike(prefix)+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("list plugin kv keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan plugin kv key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin kv keys: %w", err)
	}
	return keys, nil
}

func encodeValue(key string, value any) ([]byte, int, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, 0, fmt.Errorf("encode plugin kv value: %w", err)
	}
	return encoded, len(key) + len(encoded), nil
}

func escapeLike(raw string) string {
	raw = strings.ReplaceAll(raw, `\`, `\\`)
	raw = strings.ReplaceAll(raw, `%`, `\%`)
	raw = strings.ReplaceAll(raw, `_`, `\_`)
	return raw
}
