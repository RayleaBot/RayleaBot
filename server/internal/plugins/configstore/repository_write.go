package configstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) SeedDefaults(ctx context.Context, pluginID string, values map[string]any) (bool, error) {
	if len(values) == 0 {
		return false, nil
	}

	namespace := namespaceForPlugin(pluginID)
	existing, err := r.readQ.CountNamespace(ctx, namespace)
	if err != nil {
		return false, fmt.Errorf("count system configs for %s: %w", namespace, err)
	}
	if existing > 0 {
		return false, nil
	}

	if _, err := r.writeValues(ctx, namespace, values, false); err != nil {
		return false, err
	}
	return true, nil
}

func (r *SQLiteRepository) Write(ctx context.Context, pluginID string, values map[string]any) ([]string, error) {
	namespace := namespaceForPlugin(pluginID)
	return r.writeValues(ctx, namespace, values, true)
}

func (r *SQLiteRepository) writeValues(ctx context.Context, namespace string, values map[string]any, overwrite bool) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}

	keys := sortedConfigKeys(values)
	if len(keys) == 0 {
		return []string{}, nil
	}

	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin system config tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := r.writeQ.WithTx(tx)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	for _, key := range keys {
		raw, err := json.Marshal(values[key])
		if err != nil {
			return nil, fmt.Errorf("marshal system config %s: %w", key, err)
		}
		if overwrite {
			if err := q.UpsertConfig(ctx, sqlcgen.UpsertConfigParams{
				Namespace: namespace,
				Key:       key,
				ValueJson: string(raw),
				UpdatedAt: now,
			}); err != nil {
				return nil, fmt.Errorf("upsert system config %s: %w", key, err)
			}
		} else {
			if err := q.SeedConfig(ctx, sqlcgen.SeedConfigParams{
				Namespace: namespace,
				Key:       key,
				ValueJson: string(raw),
				UpdatedAt: now,
			}); err != nil {
				return nil, fmt.Errorf("seed system config %s: %w", key, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit system config tx: %w", err)
	}
	return keys, nil
}

func sortedConfigKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func namespaceForPlugin(pluginID string) string {
	return fmt.Sprintf("plugin:%s:settings", strings.TrimSpace(pluginID))
}
