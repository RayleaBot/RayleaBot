package dispatch

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
)

func (d *Dispatcher) executeAction(ctx context.Context, pluginID string, requestID string, event chatevent.Event, action chatevent.MessageCommand) {
	_, _ = d.ExecuteOutboundAction(ctx, pluginID, requestID, event, action)
}

// ExecuteOutboundAction sends one plugin message action through the shared
// permission, rate-limit, metrics, and outbound logging path.
func (d *Dispatcher) ExecuteOutboundAction(ctx context.Context, pluginID string, requestID string, event chatevent.Event, action chatevent.MessageCommand) (outbound.SendResult, error) {
	if d == nil || d.sender == nil {
		return outbound.SendResult{DeliveryKind: action.Kind}, &chatevent.SendError{
			Code:    errorcodes.AdapterSendFailed,
			Message: "adapter outbound sender is not available",
		}
	}

	commandName := commandNameForEvent(event)
	targetType := action.TargetType
	targetID := action.TargetID
	if event.Target != nil {
		if strings.TrimSpace(targetType) == "" {
			targetType = event.Target.Type
		}
		if strings.TrimSpace(targetID) == "" {
			targetID = event.Target.ID
		}
	}
	attempt := outbound.SendAttempt{
		ActionKind:     action.Kind,
		SourceAdapter:  action.SourceAdapter,
		SourceProtocol: action.SourceProtocol,
		TargetType:     targetType,
		TargetID:       targetID,
		Segments:       toOutboundSegments(action.MessageSegments),
	}
	targetLabel := buildOutboundTargetLabel(ctx, event, targetType, targetID, d.sender)
	if !d.permissionDeclared(ctx, pluginID, action.Kind) {
		err := &chatevent.SendError{
			Code:    errorcodes.PluginPermissionDenied,
			Message: action.Kind + " permission is not declared",
		}
		result := outbound.SendResult{
			DeliveryKind: action.Kind,
			TargetType:   targetType,
			TargetID:     targetID,
		}
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			TargetLabel: targetLabel,
		}, attempt, result, err)
		return result, err
	}
	limitTargetType, limitTargetID, limitScope := d.limitTargetForAction(action)
	if strings.TrimSpace(limitTargetType) == "" {
		limitTargetType = targetType
	}
	if strings.TrimSpace(limitTargetID) == "" {
		limitTargetID = targetID
	}
	limitRequest := outbound.MessageLimitRequest{
		Scope:      limitScope,
		PluginID:   pluginID,
		TargetType: limitTargetType,
		TargetID:   limitTargetID,
	}
	admission, err := d.beginOutboundSend(ctx, limitRequest)
	if admission.Scope.SourceAdapter != "" {
		attempt.SourceAdapter, attempt.SourceProtocol = admission.Scope.SourceAdapter, admission.Scope.SourceProtocol
	}
	if err != nil {
		result := outbound.SendResult{
			DeliveryKind: action.Kind,
			TargetType:   limitTargetType,
			TargetID:     limitTargetID,
		}
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			TargetLabel: targetLabel,
		}, attempt, result, err)
		return result, err
	}
	if admission.Scope.SourceAdapter != "" {
		action.SourceAdapter = admission.Scope.SourceAdapter
		action.SourceProtocol = admission.Scope.SourceProtocol
	}
	outboundStart := time.Now()
	result, err := outbound.SendAction(ctx, d.sender, d.resolver, event, action)
	if admission.Record != nil {
		admission.Record(err)
	}
	d.recordOutboundMetric(action, result, err, time.Since(outboundStart))
	outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
		PluginID:    pluginID,
		RequestID:   requestID,
		CommandName: commandName,
		TargetLabel: targetLabel,
	}, attempt, result, err)
	return result, err
}

func (d *Dispatcher) beginOutboundSend(ctx context.Context, request outbound.MessageLimitRequest) (outbound.MessageAdmission, error) {
	d.mu.RLock()
	policy := d.outboundPolicy
	d.mu.RUnlock()
	if policy == nil {
		return outbound.MessageAdmission{Scope: request.Scope}, nil
	}
	return policy.Begin(ctx, request)
}

func (d *Dispatcher) permissionDeclared(ctx context.Context, pluginID string, permission string) bool {
	d.mu.RLock()
	checker := d.permissionChecker
	d.mu.RUnlock()
	if checker == nil {
		return false
	}
	return checker(ctx, pluginID, permission)
}

func (d *Dispatcher) limitTargetForAction(action chatevent.MessageCommand) (string, string, chatevent.IdentityScope) {
	if action.Kind == "message.reply" && d != nil && d.resolver != nil {
		if target, ok := d.resolver.ResolveReplyTarget(strings.TrimSpace(action.ReplyToEventID)); ok {
			return target.TargetType, target.TargetID, chatevent.IdentityScope{Kind: "instance", SourceAdapter: target.SourceAdapter, SourceProtocol: target.SourceProtocol, BotID: target.BotID}
		}
	}
	return action.TargetType, action.TargetID, chatevent.IdentityScope{Kind: "instance", SourceAdapter: action.SourceAdapter, SourceProtocol: action.SourceProtocol}
}

func commandNameForEvent(event chatevent.Event) string {
	if event.PayloadFields == nil {
		return ""
	}

	commandName, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(commandName)
}

func buildOutboundTargetLabel(ctx context.Context, event chatevent.Event, targetType, targetID string, sender outbound.ActionSender) string {
	targetName := ""
	if event.Target != nil &&
		strings.TrimSpace(event.Target.Type) == strings.TrimSpace(targetType) &&
		strings.TrimSpace(event.Target.ID) == strings.TrimSpace(targetID) {
		targetName = strings.TrimSpace(event.Target.Name)
	}

	actorID := ""
	actorNickname := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorNickname = strings.TrimSpace(event.Actor.Nickname)
	}

	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}

	return outbound.BuildTargetLabel(ctx, event.SourceAdapter, targetType, targetID, targetName, actorID, actorNickname, resolver)
}

func toOutboundSegments(segments []chatevent.MessageSegment) []chatevent.MessageSegment {
	if len(segments) == 0 {
		return nil
	}

	items := make([]chatevent.MessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, chatevent.MessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}

// recordOutboundMetric records a protocol label from the resolved sender.
func (d *Dispatcher) recordOutboundMetric(action chatevent.MessageCommand, result outbound.SendResult, err error, duration time.Duration) {
	observer := d.currentMetrics()
	if observer == nil {
		return
	}
	protocol := result.SourceProtocol
	if protocol == "" {
		protocol = action.SourceProtocol
	}
	label := chatevent.ProtocolLabel(protocol)
	observer.ObserveOutboundDuration(label, duration)
	observer.IncOutboundSend(label, chatevent.SendOutcome(err))
}
