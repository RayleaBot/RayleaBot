package bridge

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func (b *Bridge) HandleAdapterEvent(ctx context.Context, event onebot11.NormalizedEvent) Outcome {
	now := time.Now().UTC()

	if !isSupportedEvent(event) {
		b.recordIgnored(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		b.logger.Debug(bridgeEventSummary("ignored", event), attrs...)
		return OutcomeIgnored
	}

	if b.dispatcher == nil || !b.dispatcher.HasDeliverablePlugins() {
		b.recordIgnored(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, "reason", "no deliverable plugin runtime is registered")
		b.logger.Debug(bridgeEventSummary("ignored", event), attrs...)
		return OutcomeIgnored
	}

	runtimeEvent := runtimeEventFromAdapter(event)

	commandName := bridgeCommandName(runtimeEvent)
	results := b.dispatcher.Dispatch(ctx, runtimeEvent, commandName)
	if len(results) == 0 {
		b.recordIgnored(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, "reason", "no plugin subscription accepted the event")
		if commandName != "" {
			attrs = append(attrs, "command_name", commandName)
		}
		b.logger.Debug(bridgeEventSummary("ignored", event), attrs...)
		return OutcomeIgnored
	}

	if bridgeDispatchDelivered(results) {
		b.recordDelivered(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, bridgeDispatchLogAttrs(results)...)
		if commandName != "" {
			attrs = append(attrs, "command_name", commandName)
		}
		b.logger.Info(bridgeEventSummary("queued for dispatcher", event), attrs...)
		return OutcomeDelivered
	}

	b.recordError(event, now, codePluginInternalError, "eligible plugin runtimes did not accept the event")
	attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
	attrs = append(attrs, bridgeDispatchLogAttrs(results)...)
	attrs = append(attrs, "error_code", codePluginInternalError)
	if commandName != "" {
		attrs = append(attrs, "command_name", commandName)
	}
	b.logger.Warn(bridgeEventSummary("failed to queue for dispatcher", event), attrs...)
	return OutcomeError
}

func (b *Bridge) LogCommandPolicyRejected(event onebot11.NormalizedEvent, rejection CommandPolicyRejection) {
	if b == nil {
		return
	}

	now := time.Now().UTC()
	errorCode := strings.TrimSpace(rejection.ErrorCode)
	reason := strings.TrimSpace(rejection.Reason)
	b.recordRejected(event, now, errorCode, reason)

	attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
	if pluginID := strings.TrimSpace(rejection.PluginID); pluginID != "" {
		attrs = append(attrs, "plugin_id", pluginID)
	}
	if commandName := strings.TrimSpace(rejection.CommandName); commandName != "" {
		attrs = append(attrs, "command_name", commandName)
	}
	if policyStage := strings.TrimSpace(rejection.PolicyStage); policyStage != "" {
		attrs = append(attrs, "policy_stage", policyStage)
	}
	if errorCode != "" {
		attrs = append(attrs, "error_code", errorCode)
	}
	if reason != "" {
		attrs = append(attrs, "reason", reason)
	}
	attrs = append(attrs, "matched_plugin_ids", cloneStringSlice(rejection.MatchedPluginIDs))

	b.logger.Warn(commandPolicyRejectedSummary(rejection), attrs...)
}

func bridgeCommandName(event pluginruntime.Event) string {
	if event.PayloadFields == nil {
		return ""
	}
	command, _ := event.PayloadFields["command"].(string)
	return strings.TrimSpace(command)
}

func bridgeDispatchDelivered(results []dispatch.DeliveryResult) bool {
	for _, result := range results {
		if result.Outcome == dispatch.OutcomeDelivered {
			return true
		}
	}
	return false
}

func bridgeDispatchLogAttrs(results []dispatch.DeliveryResult) []any {
	targetCount := len(results)
	deliveredCount := 0
	droppedCount := 0
	errorCount := 0
	lastErrorCode := ""

	for _, result := range results {
		switch result.Outcome {
		case dispatch.OutcomeDelivered:
			deliveredCount++
		case dispatch.OutcomeDropped:
			droppedCount++
		case dispatch.OutcomeError:
			errorCount++
			if lastErrorCode == "" && strings.TrimSpace(result.ErrorCode) != "" {
				lastErrorCode = result.ErrorCode
			}
		}
	}

	attrs := []any{
		"target_count", targetCount,
		"queued_count", deliveredCount,
		"dropped_count", droppedCount,
		"failed_count", errorCount,
	}
	if lastErrorCode != "" {
		attrs = append(attrs, "dispatch_error_code", lastErrorCode)
	}
	return attrs
}

func runtimeEventFromAdapter(event onebot11.NormalizedEvent) pluginruntime.Event {
	runtimeEvent := pluginruntime.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &pluginruntime.EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &pluginruntime.EventTarget{
			Type: bridgeTargetType(event),
			ID:   bridgeTargetID(event),
			Name: event.TargetName,
		},
		PayloadFields: event.PayloadFields,
		MessageID:     event.MessageID,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		runtimeEvent.Message = &pluginruntime.EventMessage{
			PlainText: event.PlainText,
			Segments:  runtimeSegmentsFromAdapter(event.Segments),
		}
	}
	return runtimeEvent
}

func runtimeSegmentsFromAdapter(segments []onebot11.MessageSegment) []pluginruntime.EventSegment {
	if len(segments) == 0 {
		return nil
	}
	projected := make([]pluginruntime.EventSegment, 0, len(segments))
	for _, seg := range segments {
		projected = append(projected, pluginruntime.EventSegment{
			Type: seg.Type,
			Data: seg.Data,
		})
	}
	return projected
}

func bridgeTargetType(event onebot11.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetType) != "" {
		return event.TargetType
	}
	return event.ConversationType
}

func bridgeTargetID(event onebot11.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetID) != "" {
		return event.TargetID
	}
	return event.ConversationID
}

func isSupportedEvent(event onebot11.NormalizedEvent) bool {
	if event.EventID == "" || event.SourceProtocol != "onebot11" || event.SourceAdapter != "adapter.onebot11" {
		return false
	}
	if event.Timestamp <= 0 || event.ConversationType == "" || event.ConversationID == "" || event.SenderID == "" {
		return false
	}
	if !isSupportedEventKind(event.Kind) {
		return false
	}
	if !isSupportedEventType(event) {
		return false
	}
	if isMessageEventKind(event.Kind) && event.PlainText == "" && len(event.Segments) == 0 {
		return false
	}
	return true
}

func isSupportedEventKind(kind string) bool {
	switch kind {
	case onebot11.EventKindMessageText, onebot11.EventKindMessage, onebot11.EventKindMessageSent, onebot11.EventKindNotice, onebot11.EventKindRequest, onebot11.EventKindMeta:
		return true
	default:
		return false
	}
}

func isMessageEventKind(kind string) bool {
	return kind == onebot11.EventKindMessageText || kind == onebot11.EventKindMessage || kind == onebot11.EventKindMessageSent
}

func isSupportedEventType(event onebot11.NormalizedEvent) bool {
	switch event.EventType {
	case "message.group":
		return event.ConversationType == "group"
	case "message.private":
		return event.ConversationType == "private"
	case "message_sent.group":
		return event.ConversationType == "group"
	case "message_sent.private":
		return event.ConversationType == "private"
	case "notice.member_increase",
		"notice.member_decrease",
		"notice.group_admin",
		"notice.group_ban",
		"notice.group_recall",
		"notice.group_upload",
		"notice.group_card",
		"notice.group_title",
		"notice.group_essence",
		"notice.group_message_emoji_like":
		return event.ConversationType == "group"
	case "notice.friend_add", "notice.friend_recall", "notice.profile_like", "notice.input_status":
		return event.ConversationType == "private"
	case "notice.poke", "notice.poke_recall", "notice.flash_file":
		return event.ConversationType == "group" || event.ConversationType == "private"
	case "request.friend":
		return event.ConversationType == "private"
	case "request.group":
		return event.ConversationType == "group"
	case "meta.heartbeat", "meta.lifecycle":
		return event.ConversationType == "system"
	default:
		return false
	}
}
