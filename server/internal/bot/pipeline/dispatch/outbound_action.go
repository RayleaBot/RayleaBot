package dispatch

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func (d *Dispatcher) executeAction(ctx context.Context, pluginID string, requestID string, event chatevent.Event, action chatevent.MessageCommand) {
	_, _ = d.ExecuteOutboundAction(ctx, pluginID, requestID, event, action)
}

// ExecuteOutboundAction sends one plugin message action through the shared
// rate-limit, metrics, and outbound logging path.
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
		Segments:       chatevent.CloneMessageSegments(action.MessageSegments),
	}
	targetLabel := buildOutboundTargetLabel(ctx, event, targetType, targetID, d.sender)
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
		botID, botNickname := buildOutboundBotIdentity(attempt.SourceAdapter, event, d.sender)
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			BotID:       botID,
			BotNickname: botNickname,
			TargetLabel: targetLabel,
		}, attempt, result, err)
		return result, err
	}
	if admission.Scope.SourceAdapter != "" {
		action.SourceAdapter = admission.Scope.SourceAdapter
		action.SourceProtocol = admission.Scope.SourceProtocol
	}
	result, err := outbound.SendAction(ctx, d.sender, d.resolver, event, action)
	deliveredBy := strings.TrimSpace(result.SourceAdapter)
	if deliveredBy == "" {
		deliveredBy = attempt.SourceAdapter
	}
	botID, botNickname := buildOutboundBotIdentity(deliveredBy, event, d.sender)
	outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
		PluginID:    pluginID,
		RequestID:   requestID,
		CommandName: commandName,
		BotID:       botID,
		BotNickname: botNickname,
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

// buildOutboundBotIdentity names the account the message goes out as. The
// event's bot id only stands in when the message leaves through the adapter
// that produced the event; another adapter is signed in as someone else.
func buildOutboundBotIdentity(adapterID string, event chatevent.Event, sender outbound.ActionSender) (string, string) {
	var resolver outbound.BotDisplayResolver
	if candidate, ok := any(sender).(outbound.BotDisplayResolver); ok {
		resolver = candidate
	}

	eventBotID := ""
	if adapterID = strings.TrimSpace(adapterID); adapterID == "" || adapterID == strings.TrimSpace(event.SourceAdapter) {
		eventBotID = event.BotID
	}
	return outbound.ResolveBotIdentity(adapterID, eventBotID, "", resolver)
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
