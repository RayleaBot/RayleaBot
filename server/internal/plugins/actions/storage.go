package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/storageaction"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
)

func (s *Service) executeStorageKV(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	req := storageaction.Request{
		PluginID: pluginID,
		Action:   action,
	}
	if s != nil {
		req.Config = s.config()
		req.Grants = s.grants
		req.KV = s.pluginKV
	}
	return storageaction.ExecuteKV(ctx, req)
}
