package actions

import (
	"context"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type ActionRequest struct {
	PluginID    string
	RequestID   string
	Action      runtimeaction.Action
	ParentEvent runtimeprotocol.Event
}

type ActionHandler func(context.Context, ActionRequest) (map[string]any, error)

type Registrar interface {
	RegisterActions(*Registry, Deps)
}

type RegistrarFunc func(*Registry, Deps)

func (fn RegistrarFunc) RegisterActions(registry *Registry, deps Deps) {
	if fn != nil {
		fn(registry, deps)
	}
}

type Registry struct {
	handlers map[string]ActionHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]ActionHandler)}
}

func DefaultRegistry() *Registry {
	return NewRegistry()
}

func NewRegistryWithRegistrars(deps Deps, registrars ...Registrar) *Registry {
	registry := NewRegistry()
	for _, registrar := range registrars {
		if registrar != nil {
			registrar.RegisterActions(registry, deps)
		}
	}
	return registry
}

func (r *Registry) Register(kind string, handler ActionHandler) {
	if r == nil || kind == "" || handler == nil {
		return
	}
	r.handlers[kind] = handler
}

func (r *Registry) Dispatch(ctx context.Context, req ActionRequest) (map[string]any, bool, error) {
	if r == nil {
		return nil, false, nil
	}
	handler, ok := r.handlers[req.Action.Kind]
	if !ok {
		return nil, false, nil
	}
	result, err := handler(ctx, req)
	return result, true, err
}

func (s *Service) Execute(ctx context.Context, pluginID, requestID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
	if s != nil && s.actionRegistry != nil {
		result, handled, err := s.actionRegistry.Dispatch(ctx, ActionRequest{
			PluginID:    pluginID,
			RequestID:   requestID,
			Action:      action,
			ParentEvent: parentEvent,
		})
		if handled {
			return result, err
		}
	}
	return nil, &runtimemanager.Error{
		Code:    "plugin.protocol_violation",
		Message: "received unsupported local action kind",
	}
}
