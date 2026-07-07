package outbound

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/redact"
)

type SendAttempt struct {
	ActionKind string
	TargetType string
	TargetID   string
	Segments   []onebot11.OutboundMessageSegment
}

type SendLogContext struct {
	PluginID    string
	RequestID   string
	CommandName string
	TargetLabel string
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

	plainText := strings.TrimSpace(onebot11.OutboundSegmentsToPlainText(attempt.Segments))
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
			sendSummary(SendLogContext{
				PluginID:    pluginID,
				CommandName: commandName,
				TargetLabel: strings.TrimSpace(context.TargetLabel),
			}, targetType, targetID, plainText, false),
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
		sendSummary(SendLogContext{
			PluginID:    pluginID,
			CommandName: commandName,
			TargetLabel: strings.TrimSpace(context.TargetLabel),
		}, targetType, targetID, plainText, true),
		fields...,
	)
}

func errorDetails(err error) (string, string) {
	var adapterErr *onebot11.Error
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

func sendSummary(context SendLogContext, targetType, targetID, plainText string, failed bool) string {
	subject := "系统"
	if pluginID := strings.TrimSpace(context.PluginID); pluginID != "" {
		subject = pluginID
		if commandName := strings.TrimSpace(context.CommandName); commandName != "" {
			subject += "/" + commandName
		}
	}

	targetLabel := strings.TrimSpace(context.TargetLabel)
	if targetLabel == "" {
		targetLabel = formatTargetLabel(targetType, targetID, "")
	}

	if failed {
		return subject + " -> " + targetLabel + " 发送失败：" + summarizePlainText(plainText)
	}
	return subject + " -> " + targetLabel + "：" + summarizePlainText(plainText)
}

type TargetDisplayResolver interface {
	ResolveTargetName(context.Context, string, string) string
}

func BuildTargetLabel(
	ctx context.Context,
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
			targetName = strings.TrimSpace(redact.SanitizeString(resolver.ResolveTargetName(ctx, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, targetName)
	case "private":
		displayName := ""
		if actorID != "" && actorID == targetID {
			displayName = actorNickname
		}
		if displayName == "" && resolver != nil {
			displayName = strings.TrimSpace(redact.SanitizeString(resolver.ResolveTargetName(ctx, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, displayName)
	default:
		if targetName == "" && resolver != nil {
			targetName = strings.TrimSpace(redact.SanitizeString(resolver.ResolveTargetName(ctx, targetType, targetID)))
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

func summarizePlainText(plainText string) string {
	plainText = strings.TrimSpace(plainText)
	if plainText == "" {
		return "[空消息]"
	}
	return redact.TruncateRunes(plainText, 72, "...")
}

func cloneOutboundSegments(segments []onebot11.OutboundMessageSegment) []map[string]any {
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
