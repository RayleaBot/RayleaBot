package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

const maxPendingServiceCalls = 64
const maxServiceCallDuration = 30 * time.Second

var serviceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

func parseServiceCallAction(raw json.RawMessage) (*plugins.Action, error) {
	if _, err := decodeAllowedActionKeys(raw, "plugin.call", "target_plugin_id", "service", "service_version", "method", "params"); err != nil {
		return nil, err
	}
	var frame pluginwire.ProtocolActionPluginCallFrame
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin.call arguments are invalid", nil)
	}
	if frame.TargetPluginID == "" || utf8.RuneCountInString(frame.TargetPluginID) > 128 || !serviceNamePattern.MatchString(frame.Service) || !serviceNamePattern.MatchString(frame.Method) || frame.ServiceVersion < 1 || int64(frame.ServiceVersion) > 2147483647 || frame.Params == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin.call arguments are invalid", nil)
	}
	return &plugins.Action{Kind: "plugin.call", ServiceCall: &plugins.ServiceCall{
		TargetPluginID: frame.TargetPluginID, Service: frame.Service,
		ServiceVersion: frame.ServiceVersion, Method: frame.Method, Params: frame.Params,
	}}, nil
}

// CallService routes to the currently running provider; its immutable startup
// declarations and process generation are checked together under the manager lock.
func (r *Registry) CallService(ctx context.Context, caller string, call plugins.ServiceCall, origin chatevent.Event) (map[string]any, error) {
	if caller == call.TargetPluginID || origin.EventType == "plugin.request" {
		return nil, errorf(errorcodes.PluginCallChainRejected, "service calls must be one hop between distinct plugins", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, eventContextError(err)
	}
	manager, ok := r.Get(call.TargetPluginID)
	if !ok {
		return nil, errorf(errorcodes.PluginServiceUnavailable, "service provider is not running", nil)
	}
	manager.mu.RLock()
	handle := manager.proc
	if handle == nil || manager.snap.State != StateRunning {
		manager.mu.RUnlock()
		return nil, errorf(errorcodes.PluginServiceUnavailable, "service provider is not running", nil)
	}
	foundName, foundVersion, foundMethod := false, false, false
	for _, service := range handle.Spec.Services {
		if service.Name != call.Service {
			continue
		}
		foundName = true
		if service.Version == call.ServiceVersion {
			foundVersion = true
			foundMethod = slices.Contains(service.Methods, call.Method)
			break
		}
	}
	done, duration := handle.Done(), handle.Spec.EventTimeout
	manager.mu.RUnlock()
	switch {
	case !foundName:
		return nil, errorf(errorcodes.PluginServiceNotFound, "service is not exported", nil)
	case !foundVersion:
		return nil, errorf(errorcodes.PluginServiceVersionUnsupported, "service version is not exported", nil)
	case !foundMethod:
		return nil, errorf(errorcodes.PluginMethodNotFound, "service method is not exported", nil)
	}
	if duration <= 0 || duration > maxServiceCallDuration {
		duration = maxServiceCallDuration
	}
	callCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	callCtx = plugins.WithExpectedRuntimeDone(callCtx, done)
	deadline, _ := callCtx.Deadline()
	source := pluginwire.ProtocolServiceOriginFrame{
		EventID: origin.EventID, EventType: origin.EventType,
		SourceProtocol: origin.SourceProtocol, SourceAdapter: origin.SourceAdapter, BotID: origin.BotID,
	}
	if origin.Actor != nil && origin.Actor.ID != "" {
		source.Actor = &pluginwire.ProtocolActorFrame{ID: origin.Actor.ID, Nickname: origin.Actor.Nickname, Role: origin.Actor.Role}
	}
	if origin.Target != nil && origin.Target.ID != "" && origin.Target.Type != "" {
		source.Target = &pluginwire.ProtocolTargetFrame{Type: origin.Target.Type, ID: origin.Target.ID, Name: origin.Target.Name}
	}
	if origin.EventType == "scheduler.trigger" {
		source.TaskID, _ = origin.PayloadFields["task_id"].(string)
	}
	// Preserve JSON null versus empty objects/arrays across the transport.
	// Configuration snapshot cloning intentionally normalizes empty containers.
	rawParams, err := json.Marshal(call.Params)
	if err != nil || call.Params == nil {
		return nil, errorf(codePlatformInvalidRequest, "service params must be a JSON object", nil)
	}
	params, err := decodeServiceObject(rawParams)
	if err != nil {
		return nil, errorf(codePlatformInvalidRequest, "service params must be a JSON object", nil)
	}
	request := pluginwire.ProtocolServiceRequestFrame{
		CallerPluginID: caller, Service: call.Service, ServiceVersion: call.ServiceVersion,
		Method: call.Method, Params: params, DeadlineAtMs: deadline.UnixMilli(), Origin: source,
	}
	event := chatevent.Event{
		EventID: nextRuntimeRequestID(), SourceProtocol: "platform", SourceAdapter: "plugins.internal",
		EventType: "plugin.request", Timestamp: time.Now().UnixMilli(),
		PayloadFields: map[string]any{"service_request": request},
	}
	delivery, err := manager.DeliverEvent(callCtx, event)
	if err != nil {
		if ctx.Err() == nil && (manager.ProcessDone() != done || manager.Snapshot().State != StateRunning) {
			return nil, errorf(errorcodes.PluginServiceUnavailable, "service provider stopped or changed runtime", nil)
		}
		return nil, err
	}
	return delivery.Result, nil
}

func decodeServiceObject(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	err := decoder.Decode(&object)
	return object, err
}

// Stop retains the existing drain policy for ordinary events, while retiring
// incoming services and canceling outgoing calls before waiting for that drain.
// The caller holds m.mu; cancellation frames are written after releasing it.
func (m *Manager) retireServiceCallsLocked() []string {
	var requests []string
	for _, session := range m.pendingEvents {
		for _, cancel := range session.serviceCancels {
			cancel()
		}
		if session.event.EventType != "plugin.request" {
			continue
		}
		failure := errorf(errorcodes.PluginServiceUnavailable, "service provider is stopping", nil)
		m.completeEventLocked(session, plugins.Delivery{RequestID: session.requestID, ErrorCode: failure.Code, ErrorMessage: failure.Message}, failure)
		m.markEventExpiredLocked(session.requestID)
		requests = append(requests, session.requestID)
	}
	return requests
}
