package outbound

import (
	"errors"
	"log/slog"
	"strings"

	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
)

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

	plainText := strings.TrimSpace(adapteroutbound.OutboundSegmentsToPlainText(attempt.Segments))
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
	var adapterErr *adapteroutbound.Error
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
