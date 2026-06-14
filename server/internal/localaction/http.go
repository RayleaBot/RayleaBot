package localaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/localaction/httpaction"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
)

func (s *Service) executeHTTPRequest(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	req := httpaction.Request{
		PluginID: pluginID,
		Action:   action,
	}
	if s != nil {
		req.Config = s.config()
		req.Grants = s.grants
		req.ThirdParty = s.thirdParty
		req.BilibiliSession = s.bilibiliSession
	}
	return httpaction.Execute(ctx, req)
}
