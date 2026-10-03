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
	outcome, report := b.QueueAdapterEvent(ctx, event)
	report()
	return outcome
}

// QueueAdapterEvent separates queue admission from logging, allowing a caller
// to release its routing lock before reporting the accepted delivery.
func (b *Bridge) QueueAdapterEvent(ctx context.Context, event chatevent.NormalizedEvent) (chatevent.DeliveryOutcome, func()) {
	now := time.Now().UTC()
	if !isSupportedEvent(event) {
		return chatevent.DeliveryOutcomeIgnored, func() {
			b.recordIgnored(event, now)
			if b.logger.Enabled(ctx, slog.LevelDebug) {
				b.logEnabledEvent(ctx, slog.LevelDebug, bridgeEventSummary("ignored", event), event, slog.String("reason", "event shape or source is outside the supported adapter contract"))
			}
		}
	}
	if b.dispatcher == nil || !b.dispatcher.HasDeliverablePlugins() {
		return chatevent.DeliveryOutcomeIgnored, func() {
			b.recordIgnored(event, now)
			if b.logger.Enabled(ctx, slog.LevelDebug) {
				b.logEnabledEvent(ctx, slog.LevelDebug, bridgeEventSummary("ignored", event), event, slog.String("reason", "no deliverable plugin runtime is registered"))
			}
		}
	}
	runtimeEvent := chatevent.FromAdapter(event)
	commandName := bridgeCommandName(runtimeEvent)
	results := b.dispatcher.Dispatch(ctx, runtimeEvent, commandName)
	if len(results) == 0 {
		return chatevent.DeliveryOutcomeIgnored, func() {
			b.recordIgnored(event, now)
			if b.logger.Enabled(ctx, slog.LevelDebug) {
				b.logEnabledEvent(ctx, slog.LevelDebug, bridgeEventSummary("ignored", event), event, appendCommandName([]slog.Attr{slog.String("reason", "no plugin subscription accepted the event")}, commandName)...)
			}
		}
	}
	if bridgeDispatchDelivered(results) {
		return chatevent.DeliveryOutcomeDelivered, func() {
			b.recordDelivered(event, now)
			if b.logger.Enabled(ctx, slog.LevelInfo) {
				b.logEnabledEvent(ctx, slog.LevelInfo, bridgeEventSummary("queued for dispatcher", event), event, appendCommandName(bridgeDispatchLogAttrs(results), commandName)...)
			}
		}
	}
	return chatevent.DeliveryOutcomeError, func() {
		b.recordError(event, now, codePluginInternalError, "eligible plugin runtimes did not accept the event")
		if !b.logger.Enabled(ctx, slog.LevelWarn) {
			return
		}
		extra := append(bridgeDispatchLogAttrs(results), slog.String("error_code", codePluginInternalError))
		b.logEnabledEvent(ctx, slog.LevelWarn, bridgeEventSummary("failed to queue for dispatcher", event), event, appendCommandName(extra, commandName)...)
	}
}

// logEnabledEvent requires the caller to check the level before building the
// summary and extra fields. Its shared attributes also clone adapter payloads.
func (b *Bridge) logEnabledEvent(ctx context.Context, level slog.Level, message string, event chatevent.NormalizedEvent, extra ...slog.Attr) {
	eventAttrs := bridgeEventLogAttrs(event)
	attrs := make([]slog.Attr, 0, 3+len(eventAttrs)+len(extra))
	attrs = append(attrs, slog.String("component", "bridge."+chatevent.ProtocolLabel(event.SourceProtocol)), slog.String("source_protocol", event.SourceProtocol), slog.String("source_adapter", event.SourceAdapter))
	attrs = append(attrs, eventAttrs...)
	attrs = append(attrs, extra...)
	b.logger.LogAttrs(ctx, level, message, attrs...)
}

func appendCommandName(attrs []slog.Attr, commandName string) []slog.Attr {
	if commandName == "" {
		return attrs
	}
	return append(attrs, slog.String("command_name", commandName))
}

func (b *Bridge) LogCommandPolicyRejected(event chatevent.NormalizedEvent, rejection chatevent.CommandPolicyRejection) {

	now := time.Now().UTC()
	errorCode := strings.TrimSpace(rejection.ErrorCode)
	reason := strings.TrimSpace(rejection.Reason)
	b.recordRejected(event, now, errorCode, reason)
	ctx := context.Background()
	if !b.logger.Enabled(ctx, slog.LevelWarn) {
		return
	}

	var attrs []slog.Attr
	if pluginID := strings.TrimSpace(rejection.PluginID); pluginID != "" {
		attrs = append(attrs, slog.String("plugin_id", pluginID))
	}
	if commandName := strings.TrimSpace(rejection.CommandName); commandName != "" {
		attrs = append(attrs, slog.String("command_name", commandName))
	}
	if policyStage := strings.TrimSpace(rejection.PolicyStage); policyStage != "" {
		attrs = append(attrs, slog.String("policy_stage", policyStage))
	}
	if errorCode != "" {
		attrs = append(attrs, slog.String("error_code", errorCode))
	}
	if reason != "" {
		attrs = append(attrs, slog.String("reason", reason))
	}
	attrs = append(attrs, slog.Any("matched_plugin_ids", cloneStringSlice(rejection.MatchedPluginIDs)))

	b.logEnabledEvent(ctx, slog.LevelWarn, commandPolicyRejectedSummary(rejection), event, attrs...)
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

func bridgeDispatchLogAttrs(results []dispatch.DeliveryResult) []slog.Attr {
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

	attrs := []slog.Attr{
		slog.Int("target_count", targetCount),
		slog.Int("queued_count", deliveredCount),
		slog.Int("dropped_count", droppedCount),
		slog.Int("failed_count", errorCount),
	}
	if lastErrorCode != "" {
		attrs = append(attrs, slog.String("dispatch_error_code", lastErrorCode))
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
