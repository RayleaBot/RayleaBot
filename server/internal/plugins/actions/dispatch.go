package actions

import (
	"context"
	"sort"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type ActionRequest struct {
	PluginID    string
	RequestID   string
	Action      pluginruntime.Action
	ParentEvent pluginruntime.Event
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

func (s *Service) Execute(ctx context.Context, pluginID, requestID string, action pluginruntime.Action, parentEvent pluginruntime.Event) (map[string]any, error) {
	result, handled, err := s.actionRegistry.Dispatch(ctx, ActionRequest{
		PluginID: pluginID, RequestID: requestID, Action: action, ParentEvent: parentEvent,
	})
	if handled {
		return result, err
	}
	return nil, &pluginruntime.Error{Code: "plugin.protocol_violation", Message: "received unsupported local action kind"}
}
