package kvstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) Delete(ctx context.Context, pluginID, key string) (bool, error) {
	result, err := r.writeQ.DeleteKV(ctx, sqlcgen.DeleteKVParams{
		PluginID: strings.TrimSpace(pluginID),
		Key:      strings.TrimSpace(key),
	})
	if err != nil {
		return false, fmt.Errorf("delete plugin kv value: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read plugin kv delete result: %w", err)
	}
	return rows > 0, nil
}
