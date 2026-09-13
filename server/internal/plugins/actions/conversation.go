package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func conversationRegistrars() []registrar {
	var result []registrar
	for _, kind := range []string{"session.wait", "session.finish"} {
		result = append(result, registrar{kind: kind, factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				if deps.Conversations == nil {
					return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "conversation registry unavailable"}
				}
				owner := conversation.Owner{PluginID: req.PluginID, Done: plugins.RuntimeDone(ctx)}
				parentID := plugins.ParentRequestID(ctx)
				if ctx.Err() != nil || parentID == "" || !owner.Alive() {
					return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "会话动作需要当前插件的活动父事件"}
				}
				if req.Action.Kind == "session.finish" {
					return map[string]any{"finished": deps.Conversations.Finish(owner, req.Action.SessionID)}, nil
				}
				ref, err := deps.Conversations.Wait(ctx, owner, parentID, req.ParentEvent, conversation.WaitRequest{SessionID: req.Action.SessionID, Scope: req.Action.SessionScope, TimeoutSeconds: req.Action.SessionTimeoutSeconds, NotifyOnExpire: req.Action.SessionNotifyOnExpire})
				if err != nil {
					return nil, err
				}
				return map[string]any{"session_id": ref.SessionID, "scope": ref.Scope, "expires_at_ms": ref.ExpiresAtMS}, nil
			}
		}})
	}
	return result
}
