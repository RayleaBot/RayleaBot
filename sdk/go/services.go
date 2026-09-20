package rayleabot

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"
)

type ServiceCallRequest = pluginwire.ProtocolActionPluginCallFrame
type ServiceRequest = pluginwire.ProtocolServiceRequestFrame
type ServiceOrigin = pluginwire.ProtocolServiceOriginFrame

type ServiceHandler func(context.Context, *EventContext, ServiceRequest) (map[string]any, error)

// Service registers handlers for a service also declared in the plugin manifest.
// Run copies registrations; they cannot change during that process session.
type Service struct {
	Name    string
	Version int
	Methods map[string]ServiceHandler
}

type serviceMethod struct {
	name    string
	version int
	method  string
}

var serviceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

func compileServices(services []Service) (map[serviceMethod]ServiceHandler, error) {
	handlers := make(map[serviceMethod]ServiceHandler)
	seen := make(map[serviceMethod]bool)
	if len(services) > 32 {
		return nil, errors.New("rayleabot: too many service declarations")
	}
	for _, service := range services {
		key := serviceMethod{name: service.Name, version: service.Version}
		if !serviceNamePattern.MatchString(service.Name) || service.Version < 1 || int64(service.Version) > 2147483647 || len(service.Methods) == 0 || len(service.Methods) > 64 || seen[key] {
			return nil, errors.New("rayleabot: invalid or duplicate service declaration")
		}
		seen[key] = true
		for method, handler := range service.Methods {
			if !serviceNamePattern.MatchString(method) || handler == nil {
				return nil, errors.New("rayleabot: service methods require a valid name and handler")
			}
			handlers[serviceMethod{name: service.Name, version: service.Version, method: method}] = handler
		}
	}
	return handlers, nil
}

func (actions *Actions) CallService(ctx context.Context, request ServiceCallRequest, output any) error {
	if request.TargetPluginID == "" || utf8.RuneCountInString(request.TargetPluginID) > 128 || !serviceNamePattern.MatchString(request.Service) || !serviceNamePattern.MatchString(request.Method) || request.ServiceVersion < 1 || int64(request.ServiceVersion) > 2147483647 || request.Params == nil {
		return errors.New("rayleabot: invalid service call request")
	}
	return actions.Call(ctx, "plugin.call", request, output)
}

func (state *runtimeState) handleService(ctx context.Context, event *EventContext) error {
	request := event.Event.ServiceRequest
	if request == nil {
		return event.Fail("plugin.protocol_violation", "service request metadata is required")
	}
	handler := state.services[serviceMethod{name: request.Service, version: request.ServiceVersion, method: request.Method}]
	if handler == nil {
		return event.Fail("plugin.method_not_found", "service method has no registered handler")
	}
	result, err := handler(ctx, event, *request)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		var failure *ActionError
		if errors.As(err, &failure) {
			return event.FailDetails(failure.Code, failure.Message, failure.Details)
		}
		// Ordinary errors may contain request or upstream response bodies.
		// Providers opt into public errors explicitly through ActionError.
		state.logger.Error("plugin service handler failed", "request_id", event.RequestID, "error_type", fmt.Sprintf("%T", err))
		return event.Fail("plugin.internal_error", "service handler failed")
	}
	if result == nil {
		result = map[string]any{}
	}
	return event.Result(result)
}

func (state *runtimeState) cancelService(requestID string) {
	state.serviceMu.Lock()
	defer state.serviceMu.Unlock()
	if cancel := state.serviceCancels[requestID]; cancel != nil {
		cancel()
	}
}

func (event *EventContext) FailDetails(code, message string, details map[string]any) error {
	return event.writeTerminal(protocolFrame{Type: "error", RequestID: event.RequestID, Code: code, Message: message, Details: details})
}
