package kvstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) Get(ctx context.Context, pluginID, key string) (any, bool, error) {
	valueJSON, err := r.readQ.GetKV(ctx, sqlcgen.GetKVParams{
		PluginID: strings.TrimSpace(pluginID),
		Key:      strings.TrimSpace(key),
	})
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
