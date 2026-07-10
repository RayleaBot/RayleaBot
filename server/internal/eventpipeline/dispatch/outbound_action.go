package dispatch

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func (d *Dispatcher) executeAction(ctx context.Context, pluginID string, requestID string, event pluginruntime.Event, action pluginruntime.Action) {
	_, _ = d.ExecuteOutboundAction(ctx, pluginID, requestID, event, action)
}

// ExecuteOutboundAction sends one plugin message action through the shared
// capability, rate-limit, metrics, and outbound logging path.
func (d *Dispatcher) ExecuteOutboundAction(ctx context.Context, pluginID string, requestID string, event pluginruntime.Event, action pluginruntime.Action) (outbound.SendResult, error) {
	if d == nil || d.sender == nil {
		return outbound.SendResult{DeliveryKind: action.Kind}, &onebot11.Error{
			Code:    onebot11.ErrorCodeSendFailed,
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
		ActionKind: action.Kind,
		TargetType: targetType,
		TargetID:   targetID,
		Segments:   toOutboundSegments(action.MessageSegments),
	}
	targetLabel := buildOutboundTargetLabel(ctx, event, targetType, targetID, d.sender)
	if !d.capabilityDeclared(ctx, pluginID, action.Kind) {
		err := &onebot11.Error{
			Code:    "plugin.capability_violation",
			Message: action.Kind + " capability is not declared",
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
	limitTargetType, limitTargetID := d.limitTargetForAction(action)
	if strings.TrimSpace(limitTargetType) == "" {
		limitTargetType = targetType
	}
	if strings.TrimSpace(limitTargetID) == "" {
		limitTargetID = targetID
	}
	if err := d.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
		PluginID:   pluginID,
		TargetType: limitTargetType,
		TargetID:   limitTargetID,
	}); err != nil {
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
	outboundStart := time.Now()
	result, err := outbound.SendAction(ctx, d.sender, d.resolver, event, action)
	d.recordOutboundMetric(action, result, err, time.Since(outboundStart))
	outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
		PluginID:    pluginID,
		RequestID:   requestID,
		CommandName: commandName,
		TargetLabel: targetLabel,
	}, attempt, result, err)
	return result, err
}

func (d *Dispatcher) capabilityDeclared(ctx context.Context, pluginID string, capability string) bool {
	d.mu.RLock()
	checker := d.capabilityChecker
	d.mu.RUnlock()
	if checker == nil {
		return true
	}
	return checker(ctx, pluginID, capability)
}

func (d *Dispatcher) waitOutboundLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	d.mu.RLock()
	limiter := d.outboundLimiter
	d.mu.RUnlock()
	if limiter == nil {
		return nil
	}
	return limiter.Wait(ctx, request)
}

func (d *Dispatcher) limitTargetForAction(action pluginruntime.Action) (string, string) {
	if action.Kind == "message.reply" && d != nil && d.resolver != nil {
		if target, ok := d.resolver.ResolveReplyTarget(strings.TrimSpace(action.ReplyToEventID)); ok {
			return target.TargetType, target.TargetID
		}
	}
	return action.TargetType, action.TargetID
}

func commandNameForEvent(event pluginruntime.Event) string {
	if event.PayloadFields == nil {
		return ""
	}

	commandName, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(commandName)
}

func buildOutboundTargetLabel(ctx context.Context, event pluginruntime.Event, targetType, targetID string, sender outbound.ActionSender) string {
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

	return outbound.BuildTargetLabel(ctx, targetType, targetID, targetName, actorID, actorNickname, resolver)
}

func toOutboundSegments(segments []pluginruntime.ActionSegment) []onebot11.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}

	items := make([]onebot11.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, onebot11.OutboundMessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}

// recordOutboundMetric routes a single outbound send outcome into the
// dispatcher MetricsObserver. The adapter label is the OneBot11 shell;
// outbound currently routes through a single shared adapter, so the label
// stays bounded and predictable.
func (d *Dispatcher) recordOutboundMetric(action pluginruntime.Action, result outbound.SendResult, err error, duration time.Duration) {
	observer := d.currentMetrics()
	if observer == nil {
		return
	}
	adapterLabel := outboundAdapterLabel(action)
	observer.ObserveOutboundDuration(adapterLabel, duration)
	observer.IncOutboundSend(adapterLabel, outboundOutcome(err))
	_ = result
}

func outboundAdapterLabel(_ pluginruntime.Action) string {
	return "onebot11"
}

func outboundOutcome(err error) string {
	if err == nil {
		return "delivered"
	}
	var adapterErr *onebot11.Error
	if errors.As(err, &adapterErr) {
		switch adapterErr.Code {
		case "plugin.capability_violation":
			return "capability_violation"
		case "adapter.reply_target_missing":
			return "reply_target_missing"
		}
	}
	return "failed"
}
