package dispatch

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (d *Dispatcher) executeAction(ctx context.Context, pluginID string, requestID string, event runtime.Event, action runtime.Action) {
	if d.sender == nil {
		return
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
	if !d.capabilityGranted(ctx, pluginID, action.Kind) {
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			TargetLabel: targetLabel,
		}, attempt, outbound.SendResult{
			DeliveryKind: action.Kind,
			TargetType:   targetType,
			TargetID:     targetID,
		}, &adapter.Error{
			Code:    "permission.scope_violation",
			Message: action.Kind + " capability is not granted",
		})
		return
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
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			TargetLabel: targetLabel,
		}, attempt, outbound.SendResult{
			DeliveryKind: action.Kind,
			TargetType:   limitTargetType,
			TargetID:     limitTargetID,
		}, err)
		return
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
}
