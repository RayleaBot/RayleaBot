package localaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

type baseActionHandler func(context.Context, *Service, string, string, runtime.Action, runtime.Event) (map[string]any, error)

var baseActionHandlers = map[string]baseActionHandler{
	"logger.write": func(ctx context.Context, s *Service, pluginID, requestID string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeLoggerWrite(ctx, pluginID, requestID, action)
	},
	"storage.kv": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeStorageKV(ctx, pluginID, action)
	},
	"config.read": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeConfigRead(ctx, pluginID, action)
	},
	"plugin.list": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, parentEvent runtime.Event) (map[string]any, error) {
		return s.executePluginList(ctx, pluginID, action, parentEvent)
	},
	"secret.read": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeSecretRead(ctx, pluginID, action)
	},
	"config.write": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeConfigWrite(ctx, pluginID, action)
	},
	"governance.blacklist.read": func(ctx context.Context, s *Service, pluginID, _ string, _ runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeGovernanceBlacklistRead(ctx, pluginID)
	},
	"governance.blacklist.write": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeGovernanceBlacklistWrite(ctx, pluginID, action)
	},
	"governance.whitelist.read": func(ctx context.Context, s *Service, pluginID, _ string, _ runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeGovernanceWhitelistRead(ctx, pluginID)
	},
	"governance.whitelist.write": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeGovernanceWhitelistWrite(ctx, pluginID, action)
	},
	"governance.command_policy.read": func(ctx context.Context, s *Service, pluginID, _ string, _ runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeGovernanceCommandPolicyRead(ctx, pluginID)
	},
	"storage.file": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeStorageFile(ctx, pluginID, action)
	},
	"http.request": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeHTTPRequest(ctx, pluginID, action)
	},
	"scheduler.create": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeSchedulerCreate(ctx, pluginID, action)
	},
	"event.expose_webhook": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, _ runtime.Event) (map[string]any, error) {
		return s.executeExposeWebhook(ctx, pluginID, action)
	},
	"render.image": func(ctx context.Context, s *Service, pluginID, _ string, action runtime.Action, parentEvent runtime.Event) (map[string]any, error) {
		return s.executeRenderImage(ctx, pluginID, action, parentEvent)
	},
}

func (s *Service) Execute(ctx context.Context, pluginID, requestID string, action runtime.Action, parentEvent runtime.Event) (map[string]any, error) {
	if handler, ok := baseActionHandlers[action.Kind]; ok {
		return handler(ctx, s, pluginID, requestID, action, parentEvent)
	}
	switch {
	case runtimeIsOneBotLocalAction(action.Kind), runtimeIsProviderExtensionAction(action.Kind):
		return s.executeOneBotLocalAction(ctx, pluginID, action)
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported local action kind",
		}
	}
}
