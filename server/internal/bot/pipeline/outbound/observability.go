package outbound

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/redact"
)

type SendAttempt struct {
	SourceAdapter  string
	SourceProtocol string
	ActionKind     string
	TargetType     string
	TargetID       string
	Segments       []chatevent.MessageSegment
}

// SendLogContext is what the caller knows about a send beyond the attempt
// itself. BotID and BotNickname name the account the message goes out as;
// TargetLabel is the conversation as BuildTargetLabel renders it.
type SendLogContext struct {
	PluginID    string
	RequestID   string
	CommandName string
	BotID       string
	BotNickname string
	TargetLabel string
}

func LogSendOutcome(logger *slog.Logger, logContext SendLogContext, attempt SendAttempt, result SendResult, err error) {
	if logger == nil {
		return
	}
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelWarn
	}
	if !logger.Enabled(context.Background(), level) {
		return
	}

	targetType := strings.TrimSpace(result.TargetType)
	if targetType == "" {
		targetType = strings.TrimSpace(attempt.TargetType)
	}

	targetID := strings.TrimSpace(result.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(attempt.TargetID)
	}

	deliveryKind := strings.TrimSpace(result.DeliveryKind)
	if deliveryKind == "" {
		deliveryKind = strings.TrimSpace(attempt.ActionKind)
	}

	plainText := strings.TrimSpace(chatevent.PlainText(attempt.Segments))
	if plainText == "" {
		plainText = "[empty message]"
	}

	pluginID := strings.TrimSpace(logContext.PluginID)
	requestID := strings.TrimSpace(logContext.RequestID)
	commandName := strings.TrimSpace(logContext.CommandName)
	botID := strings.TrimSpace(logContext.BotID)
	botNickname := strings.TrimSpace(redact.SanitizeString(logContext.BotNickname))
	targetLabel := strings.TrimSpace(logContext.TargetLabel)
	if targetLabel == "" {
		targetLabel = formatTargetLabel(targetType, targetID, "")
	}
	summary := logging.OutboundMessageSummary(logging.OutboundMessageSummaryInput{
		BotID:       botID,
		BotNickname: botNickname,
		TargetLabel: targetLabel,
		PlainText:   plainText,
	})

	protocol := result.SourceProtocol
	if protocol == "" {
		protocol = attempt.SourceProtocol
	}
	adapter := result.SourceAdapter
	if adapter == "" {
		adapter = attempt.SourceAdapter
	}
	fields := make([]slog.Attr, 0, 19)
	fields = append(fields,
		slog.String("component", "adapter."+chatevent.ProtocolLabel(protocol)),
		slog.String("source_protocol", protocol),
		slog.String("source_adapter", adapter),
		slog.String("target_label", targetLabel),
		slog.String("outcome", chatevent.SendOutcome(err)),
		slog.String("direction", "outbound"),
		slog.String("action_kind", strings.TrimSpace(attempt.ActionKind)),
		slog.String("delivery_kind", deliveryKind),
		slog.String("target_type", targetType),
		slog.String("target_id", targetID),
		slog.String("plain_text", plainText),
		slog.Any("segments", cloneOutboundSegments(attempt.Segments)),
	)
	if botID != "" {
		fields = append(fields, slog.String("self_id", botID))
	}
	if botNickname != "" {
		fields = append(fields, slog.String("self_nickname", botNickname))
	}
	if pluginID != "" {
		fields = append(fields, slog.String("plugin_id", pluginID))
	}
	if requestID != "" {
		fields = append(fields, slog.String("request_id", requestID))
	}
	if commandName != "" {
		fields = append(fields, slog.String("command_name", commandName))
	}

	if err == nil {
		if messageID := strings.TrimSpace(result.MessageID); messageID != "" {
			fields = append(fields, slog.String("message_id", messageID))
		}
		logger.LogAttrs(context.Background(), slog.LevelInfo, "消息已发送："+summary, fields...)
		return
	}

	errorCode, reason := errorDetails(err)
	if errorCode != "" {
		fields = append(fields, slog.String("error_code", errorCode))
	}
	fields = append(fields, slog.String("reason", reason))
	if errorCode == errorcodes.AdapterSendUnconfirmed {
		logger.LogAttrs(context.Background(), slog.LevelWarn, "消息发送状态未确认，不自动重发："+summary, fields...)
		return
	}
	logger.LogAttrs(context.Background(), slog.LevelWarn, "消息发送失败："+summary, fields...)
}

func errorDetails(err error) (string, string) {
	var adapterErr interface {
		RuntimeActionCode() string
		RuntimeActionMessage() string
	}
	if errors.As(err, &adapterErr) {
		reason := strings.TrimSpace(adapterErr.RuntimeActionMessage())
		if reason == "" {
			reason = strings.TrimSpace(err.Error())
		}
		return strings.TrimSpace(adapterErr.RuntimeActionCode()), reason
	}

	reason := strings.TrimSpace(err.Error())
	if reason == "" {
		reason = "unknown outbound error"
	}
	return "", reason
}

// TargetDisplayResolver names a conversation for a log line. The adapter id is
// part of the question: target identifiers live in each adapter's own
// namespace, so asking the wrong one either answers nothing or answers about an
// unrelated conversation that happens to share an id.
type TargetDisplayResolver interface {
	ResolveTargetName(ctx context.Context, adapterID, targetType, targetID string) string
}

// BotDisplayResolver names the account an adapter instance is signed in as.
// As with TargetDisplayResolver the adapter id is part of the question: each
// instance only knows its own login.
type BotDisplayResolver interface {
	ResolveBotDisplay(adapterID string) (botID string, nickname string)
}

// ResolveBotIdentity picks the account an outbound log line names: the
// adapter's own login when it can answer, otherwise the identity the
// triggering event carried.
func ResolveBotIdentity(adapterID, eventBotID, eventBotNickname string, resolver BotDisplayResolver) (string, string) {
	adapterID = strings.TrimSpace(adapterID)
	if resolver != nil && adapterID != "" {
		if botID, nickname := resolver.ResolveBotDisplay(adapterID); strings.TrimSpace(botID) != "" {
			return strings.TrimSpace(botID), strings.TrimSpace(nickname)
		}
	}
	return strings.TrimSpace(eventBotID), strings.TrimSpace(eventBotNickname)
}

func BuildTargetLabel(
	ctx context.Context,
	adapterID string,
	targetType string,
	targetID string,
	targetName string,
	actorID string,
	actorNickname string,
	resolver TargetDisplayResolver,
) string {
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	targetName = strings.TrimSpace(redact.SanitizeString(targetName))
	actorID = strings.TrimSpace(actorID)
	actorNickname = strings.TrimSpace(redact.SanitizeString(actorNickname))

	switch targetType {
	case "group":
		if targetName == "" && resolver != nil {
			targetName = strings.TrimSpace(redact.SanitizeString(resolver.ResolveTargetName(ctx, adapterID, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, targetName)
	case "private":
		displayName := ""
		if actorID != "" && actorID == targetID {
			displayName = actorNickname
		}
		if displayName == "" && resolver != nil {
			displayName = strings.TrimSpace(redact.SanitizeString(resolver.ResolveTargetName(ctx, adapterID, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, displayName)
	default:
		if targetName == "" && resolver != nil {
			targetName = strings.TrimSpace(redact.SanitizeString(resolver.ResolveTargetName(ctx, adapterID, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, targetName)
	}
}

func formatTargetLabel(targetType string, targetID string, displayName string) string {
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	displayName = strings.TrimSpace(redact.SanitizeString(displayName))

	switch targetType {
	case "group":
		if displayName != "" && targetID != "" {
			return fmt.Sprintf("[%s(%s)]", displayName, targetID)
		}
		if displayName != "" {
			return fmt.Sprintf("[%s]", displayName)
		}
		if targetID != "" {
			return fmt.Sprintf("[%s]", targetID)
		}
		return "[群聊]"
	case "private":
		if displayName != "" && targetID != "" {
			return fmt.Sprintf("%s(%s)", displayName, targetID)
		}
		if displayName != "" {
			return displayName
		}
		if targetID != "" {
			return fmt.Sprintf("私聊(%s)", targetID)
		}
		return "私聊"
	default:
		if displayName != "" && targetID != "" {
			return fmt.Sprintf("%s(%s)", displayName, targetID)
		}
		if displayName != "" {
			return displayName
		}
		if targetType != "" && targetID != "" {
			return fmt.Sprintf("%s(%s)", targetType, targetID)
		}
		if targetID != "" {
			return targetID
		}
		if targetType != "" {
			return targetType
		}
		return "未知目标"
	}
}

func cloneOutboundSegments(segments []chatevent.MessageSegment) []map[string]any {
	if len(segments) == 0 {
		return []map[string]any{}
	}

	items := make([]map[string]any, 0, len(segments))
	for _, segment := range segments {
		items = append(items, map[string]any{
			"type": strings.TrimSpace(segment.Type),
			"data": cloneOutboundSegmentData(segment.Data),
		})
	}
	return items
}

func cloneOutboundSegmentData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = cloneOutboundValue(value)
	}
	return cloned
}

func cloneOutboundValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneOutboundSegmentData(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, cloneOutboundValue(item))
		}
		return items
	default:
		return typed
	}
}
