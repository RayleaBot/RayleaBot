package dispatch

import (
	"fmt"
	"strings"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
)

func laneKeyForEvent(event runtimeprotocol.Event, fallbackCounter *int) string {
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

func commandNameForEvent(event runtimeprotocol.Event) string {
	if event.PayloadFields == nil {
		return ""
	}

	commandName, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(commandName)
}
