package dispatch

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func laneKeyForEvent(event runtime.Event, fallbackCounter *int) string {
	if event.Target != nil {
		targetType := strings.TrimSpace(event.Target.Type)
		targetID := strings.TrimSpace(event.Target.ID)
		if targetType != "" && targetID != "" {
			return targetType + ":" + targetID
		}
	}
	*fallbackCounter = *fallbackCounter + 1
	return fmt.Sprintf("fallback:%d", *fallbackCounter)
}

func commandNameForEvent(event runtime.Event) string {
	if event.PayloadFields == nil {
		return ""
	}

	commandName, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(commandName)
}
