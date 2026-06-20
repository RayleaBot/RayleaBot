package dispatch

import (
	"context"
	"strings"
	"time"

	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (d *Dispatcher) executeAction(ctx context.Context, pluginID string, requestID string, event runtimeprotocol.Event, action runtimeaction.Action) {
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
	if !d.capabilityDeclared(ctx, pluginID, action.Kind) {
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			TargetLabel: targetLabel,
		}, attempt, outbound.SendResult{
			DeliveryKind: action.Kind,
			TargetType:   targetType,
			TargetID:     targetID,
		}, &adapteroutbound.Error{
			Code:    "plugin.capability_violation",
			Message: action.Kind + " capability is not declared",
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
