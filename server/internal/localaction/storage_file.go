package localaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/localaction/storageaction"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
)

func (s *Service) executeStorageFile(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	req := storageaction.Request{
		PluginID: pluginID,
		Action:   action,
	}
	if s != nil {
		req.Config = s.config()
		req.Grants = s.grants
		req.Files = s.pluginFiles
	}
	return storageaction.ExecuteFile(ctx, req)
}
