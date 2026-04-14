package outbound

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

type SendAttempt struct {
	ActionKind string
	TargetType string
	TargetID   string
	Segments   []adapter.OutboundMessageSegment
}

type SendLogContext struct {
	PluginID    string
	RequestID   string
	CommandName string
}

func LogSendOutcome(logger *slog.Logger, context SendLogContext, attempt SendAttempt, result SendResult, err error) {
	if logger == nil {
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

	plainText := strings.TrimSpace(adapter.OutboundSegmentsToPlainText(attempt.Segments))
	if plainText == "" {
		plainText = "[empty message]"
	}

	pluginID := strings.TrimSpace(context.PluginID)
	requestID := strings.TrimSpace(context.RequestID)
	commandName := strings.TrimSpace(context.CommandName)

	fields := []any{
		"component", "adapter.onebot11",
		"direction", "outbound",
		"action_kind", strings.TrimSpace(attempt.ActionKind),
		"delivery_kind", deliveryKind,
		"target_type", targetType,
		"target_id", targetID,
		"plain_text", plainText,
		"segments", cloneOutboundSegments(attempt.Segments),
	}
	if pluginID != "" {
		fields = append(fields, "plugin_id", pluginID)
	}
	if requestID != "" {
		fields = append(fields, "request_id", requestID)
	}
	if commandName != "" {
		fields = append(fields, "command_name", commandName)
	}

	if err == nil {
		if messageID := strings.TrimSpace(result.MessageID); messageID != "" {
			fields = append(fields, "message_id", messageID)
		}
		logger.Info(
			sendSummary(summaryPrefix(pluginID, commandName, false), targetType, plainText),
			fields...,
		)
		return
	}

	errorCode, reason := errorDetails(err)
	if errorCode != "" {
		fields = append(fields, "error_code", errorCode)
	}
	fields = append(fields, "reason", reason)
	logger.Warn(
		sendSummary(summaryPrefix(pluginID, commandName, true), targetType, plainText),
		fields...,
	)
}

func summaryPrefix(pluginID, commandName string, failed bool) string {
	if pluginID == "" {
		if failed {
			return "platform failed to deliver"
		}
		return "platform delivered"
	}

	if commandName != "" {
		if failed {
			return "plugin " + pluginID + " command " + commandName + " failed to deliver"
		}
		return "plugin " + pluginID + " command " + commandName + " delivered"
	}

	if failed {
		return "plugin " + pluginID + " failed to deliver"
	}
	return "plugin " + pluginID + " delivered"
}

func sendSummary(prefix, targetType, plainText string) string {
	targetType = strings.TrimSpace(targetType)
	if targetType == "" {
		targetType = "unknown"
	}
	return prefix + " " + targetType + " message: " + summarizePlainText(plainText)
}

func summarizePlainText(plainText string) string {
	plainText = strings.TrimSpace(plainText)
	if plainText == "" {
		return "[empty message]"
	}
	return textsafe.TruncateRunes(plainText, 72, "...")
}

func errorDetails(err error) (string, string) {
	var adapterErr *adapter.Error
	if errors.As(err, &adapterErr) {
		reason := strings.TrimSpace(adapterErr.Message)
		if reason == "" {
			reason = strings.TrimSpace(adapterErr.Error())
		}
		return strings.TrimSpace(adapterErr.Code), reason
	}

	reason := strings.TrimSpace(err.Error())
	if reason == "" {
		reason = "unknown outbound error"
	}
	return "", reason
}

func cloneOutboundSegments(segments []adapter.OutboundMessageSegment) []map[string]any {
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
