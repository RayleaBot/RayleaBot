package bridge

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func bridgeCommandName(event runtime.Event) string {
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
