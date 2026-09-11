package bridge

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
)

func (b *Bridge) HandleAdapterEvent(ctx context.Context, event chatevent.NormalizedEvent) chatevent.DeliveryOutcome {
	now := time.Now().UTC()

	if !isSupportedEvent(event) {
		b.recordIgnored(event, now)
		b.logEvent(ctx, slog.LevelDebug, bridgeEventSummary("ignored", event), event, "reason", "event shape or source is outside the supported adapter contract")
		return chatevent.DeliveryOutcomeIgnored
	}

	if b.dispatcher == nil || !b.dispatcher.HasDeliverablePlugins() {
		b.recordIgnored(event, now)
		b.logEvent(ctx, slog.LevelDebug, bridgeEventSummary("ignored", event), event, "reason", "no deliverable plugin runtime is registered")
		return chatevent.DeliveryOutcomeIgnored
	}

	runtimeEvent := chatevent.FromAdapter(event)

	commandName := bridgeCommandName(runtimeEvent)
	results := b.dispatcher.Dispatch(ctx, runtimeEvent, commandName)
	if len(results) == 0 {
		b.recordIgnored(event, now)
		b.logEvent(ctx, slog.LevelDebug, bridgeEventSummary("ignored", event), event, appendCommandName([]any{"reason", "no plugin subscription accepted the event"}, commandName)...)
		return chatevent.DeliveryOutcomeIgnored
	}

	if bridgeDispatchDelivered(results) {
		b.recordDelivered(event, now)
		b.logEvent(ctx, slog.LevelInfo, bridgeEventSummary("queued for dispatcher", event), event, appendCommandName(bridgeDispatchLogAttrs(results), commandName)...)
		return chatevent.DeliveryOutcomeDelivered
	}

	b.recordError(event, now, codePluginInternalError, "eligible plugin runtimes did not accept the event")
	extra := append(bridgeDispatchLogAttrs(results), "error_code", codePluginInternalError)
	b.logEvent(ctx, slog.LevelWarn, bridgeEventSummary("failed to queue for dispatcher", event), event, appendCommandName(extra, commandName)...)
	return chatevent.DeliveryOutcomeError
}

// logEvent emits one bridge log line with the shared event attributes. The
// attribute set clones adapter payloads, so it is only built when the level
// is enabled.
func (b *Bridge) logEvent(ctx context.Context, level slog.Level, message string, event chatevent.NormalizedEvent, extra ...any) {
	if !b.logger.Enabled(ctx, level) {
		return
	}
	attrs := append([]any{"component", "bridge." + chatevent.ProtocolLabel(event.SourceProtocol), "source_protocol", event.SourceProtocol, "source_adapter", event.SourceAdapter}, bridgeEventLogAttrs(event)...)
	attrs = append(attrs, extra...)
	b.logger.Log(ctx, level, message, attrs...)
}

func appendCommandName(attrs []any, commandName string) []any {
	if commandName == "" {
		return attrs
	}
	return append(attrs, "command_name", commandName)
}

func (b *Bridge) LogCommandPolicyRejected(event chatevent.NormalizedEvent, rejection chatevent.CommandPolicyRejection) {

	now := time.Now().UTC()
	errorCode := strings.TrimSpace(rejection.ErrorCode)
	reason := strings.TrimSpace(rejection.Reason)
	b.recordRejected(event, now, errorCode, reason)

	var attrs []any
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

	b.logEvent(context.Background(), slog.LevelWarn, commandPolicyRejectedSummary(rejection), event, attrs...)
}

func bridgeCommandName(event chatevent.Event) string {
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

// supportedProtocols lists the chat protocols whose events the bridge delivers.
// An event from anything else is ignored rather than guessed at.
var supportedProtocols = map[string]bool{
	"onebot11":   true,
	"qqofficial": true,
}

func isSupportedEvent(event chatevent.NormalizedEvent) bool {
	if event.EventID == "" {
		return false
	}
	// The protocol decides how the event is read; the adapter names which
	// instance produced it, and only has to be present so a reply can be
	// routed back to that instance.
	if !supportedProtocols[event.SourceProtocol] || strings.TrimSpace(event.SourceAdapter) == "" {
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
	switch chatevent.EventFamily(kind) {
	case chatevent.FamilyMessageText, chatevent.FamilyMessage, chatevent.FamilyMessageSent,
		chatevent.FamilyNotice, chatevent.FamilyRequest, chatevent.FamilyMeta:
		return true
	default:
		return false
	}
}

func isMessageEventKind(kind string) bool {
	switch chatevent.EventFamily(kind) {
	case chatevent.FamilyMessageText, chatevent.FamilyMessage, chatevent.FamilyMessageSent:
		return true
	default:
		return false
	}
}

func isSupportedEventType(event chatevent.NormalizedEvent) bool {
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
	case "notice.bot_added", "notice.bot_removed", "notice.push_enabled", "notice.push_disabled":
		// The bot joining or leaving, and push being switched on or off, happen
		// in either kind of conversation.
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
