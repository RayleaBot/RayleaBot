package actions

import (
	"context"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type ActionRequest struct {
	PluginID    string
	RequestID   string
	Action      plugins.Action
	ParentEvent chatevent.Event
}

type ActionHandler func(context.Context, ActionRequest) (map[string]any, error)

type Registry struct{ handlers map[string]ActionHandler }

func (r *Registry) Kinds() []string {
	kinds := make([]string, 0, len(r.handlers))
	for kind := range r.handlers {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func (r *Registry) Dispatch(ctx context.Context, req ActionRequest) (map[string]any, bool, error) {
	handler, ok := r.handlers[req.Action.Kind]
	if !ok {
		return nil, false, nil
	}
	result, err := handler(ctx, req)
	return result, true, err
}

func (s *Service) Execute(ctx context.Context, pluginID, requestID string, action plugins.Action, parentEvent chatevent.Event) (map[string]any, error) {
	result, handled, err := s.actionRegistry.Dispatch(ctx, ActionRequest{
		PluginID: pluginID, RequestID: requestID, Action: action, ParentEvent: parentEvent,
	})
	if handled {
		return result, err
	}
	return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "received unsupported local action kind"}
}
