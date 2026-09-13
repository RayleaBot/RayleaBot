package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

var (
	ErrKVValueTooLarge  = errors.New("plugin kv value exceeds configured limit")
	ErrKVQuotaExceeded  = errors.New("plugin kv total capacity exceeds configured limit")
	ErrKVInvalidRequest = errors.New("invalid plugin kv request")
)

const MaxKVTTLSeconds = 31536000

type KVSetOptions struct {
	TTLSeconds int
}
type KVEntry struct {
	Value       any
	Exists      bool
	ExpiresAtMS *int64
}
type KVSetResult struct {
	ExpiresAtMS *int64
}

type KVLimits struct {
	ValueMaxBytes int
	TotalMaxBytes int
}

type KVRepository interface {
	GetEntry(context.Context, string, string) (KVEntry, error)
	SetWithOptions(context.Context, string, string, any, KVLimits, KVSetOptions) (KVSetResult, error)
	Delete(context.Context, string, string) (bool, error)
	List(context.Context, string, string) ([]string, error)
	SweepExpired(context.Context) (int64, error)
}

type KVSQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	write  *sql.DB
	read   *sql.DB
	now    func() time.Time
}

func NewKVSQLiteRepository(store *storage.Store) (*KVSQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &KVSQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		write:  store.Write,
		read:   store.Read,
		now:    time.Now,
	}, nil
}

func (r *KVSQLiteRepository) Get(ctx context.Context, pluginID, key string) (any, bool, error) {
	entry, err := r.GetEntry(ctx, pluginID, key)
	return entry.Value, entry.Exists, err
}

func (r *KVSQLiteRepository) GetEntry(ctx context.Context, pluginID, key string) (KVEntry, error) {
	row, err := r.readQ.GetKV(ctx, sqlcgen.GetKVParams{
		PluginID: strings.TrimSpace(pluginID),
		Key:      strings.TrimSpace(key),
		NowMs:    sql.NullInt64{Int64: r.now().UnixMilli(), Valid: true},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return KVEntry{}, nil
	}
	if err != nil {
		return KVEntry{}, fmt.Errorf("query plugin kv value: %w", err)
	}

	var value any
	if err := json.Unmarshal([]byte(row.ValueJson), &value); err != nil {
		return KVEntry{}, fmt.Errorf("decode plugin kv value: %w", err)
	}
	return KVEntry{Value: value, Exists: true, ExpiresAtMS: kvExpiry(row.ExpiresAtMs)}, nil
}

func (r *KVSQLiteRepository) Set(ctx context.Context, pluginID, key string, value any, limits KVLimits) error {
	_, err := r.SetWithOptions(ctx, pluginID, key, value, limits, KVSetOptions{})
	return err
}

func (r *KVSQLiteRepository) SetWithOptions(ctx context.Context, pluginID, key string, value any, limits KVLimits, options KVSetOptions) (KVSetResult, error) {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)
	if pluginID == "" || key == "" || options.TTLSeconds < 0 || options.TTLSeconds > MaxKVTTLSeconds {
		return KVSetResult{}, ErrKVInvalidRequest
	}

	valueJSON, sizeBytes, err := encodeValue(key, value)
	if err != nil {
		return KVSetResult{}, err
	}
	if limits.ValueMaxBytes > 0 && len(valueJSON) > limits.ValueMaxBytes {
		return KVSetResult{}, ErrKVValueTooLarge
	}

	var result KVSetResult
	err = storage.WithTx(ctx, r.write, nil, func(tx *sql.Tx) error {
		now := r.now()
		nowMS := sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
		q := r.writeQ.WithTx(tx)
		previousSize, err := q.GetKVSize(ctx, sqlcgen.GetKVSizeParams{
			PluginID: pluginID,
			Key:      key,
			NowMs:    nowMS,
		})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query previous plugin kv size: %w", err)
		}
		totalSize, err := q.GetKVTotalSize(ctx, nowMS)
		if err != nil {
			return fmt.Errorf("query global plugin kv total size: %w", err)
		}
		nextTotal := totalSize - previousSize + int64(sizeBytes)
		if limits.TotalMaxBytes > 0 && nextTotal > int64(limits.TotalMaxBytes) {
			return ErrKVQuotaExceeded
		}
		expiry := sql.NullInt64{}
		if options.TTLSeconds > 0 {
			expiry = sql.NullInt64{Int64: now.Add(time.Duration(options.TTLSeconds) * time.Second).UnixMilli(), Valid: true}
		}
		if err := q.UpsertKV(ctx, sqlcgen.UpsertKVParams{PluginID: pluginID, Key: key, ValueJson: string(valueJSON), SizeBytes: int64(sizeBytes), UpdatedAt: now.UTC().Format(time.RFC3339Nano), ExpiresAtMs: expiry}); err != nil {
			return fmt.Errorf("upsert plugin kv value: %w", err)
		}
		result.ExpiresAtMS = kvExpiry(expiry)
		return nil
	})
	if err != nil {
		return KVSetResult{}, err
	}
	return result, nil
}

func (r *KVSQLiteRepository) Delete(ctx context.Context, pluginID, key string) (bool, error) {
	var count int64
	err := storage.WithTx(ctx, r.write, nil, func(tx *sql.Tx) error {
		var err error
		count, err = r.writeQ.WithTx(tx).DeleteKV(ctx, sqlcgen.DeleteKVParams{PluginID: strings.TrimSpace(pluginID), Key: strings.TrimSpace(key), NowMs: sql.NullInt64{Int64: r.now().UnixMilli(), Valid: true}})
		return err
	})
	if err != nil {
		return false, fmt.Errorf("delete plugin kv value: %w", err)
	}
	return count > 0, nil
}

// List uses ESCAPE clause not supported by sqlc's SQLite parser; kept as hand-written SQL.
func (r *KVSQLiteRepository) List(ctx context.Context, pluginID, prefix string) ([]string, error) {
	rows, err := r.read.QueryContext(
		ctx,
		`SELECT key
		 FROM plugin_kv
		 WHERE plugin_id = ? AND key LIKE ? ESCAPE '\'
		 AND (expires_at_ms IS NULL OR expires_at_ms > ?)
		 ORDER BY key ASC`,
		strings.TrimSpace(pluginID),
		escapeLike(prefix)+"%",
		r.now().UnixMilli(),
	)
	if err != nil {
		return nil, fmt.Errorf("list plugin kv keys: %w", err)
	}
	defer func(release func() error) { _ = release() }(rows.Close)

	keys := []string{}
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

func kvExpiry(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
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
