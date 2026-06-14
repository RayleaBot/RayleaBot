package dispatch

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func buildOutboundTargetLabel(ctx context.Context, event runtimeprotocol.Event, targetType, targetID string, sender outbound.ActionSender) string {
	targetName := ""
	if event.Target != nil &&
		strings.TrimSpace(event.Target.Type) == strings.TrimSpace(targetType) &&
		strings.TrimSpace(event.Target.ID) == strings.TrimSpace(targetID) {
		targetName = strings.TrimSpace(event.Target.Name)
	}

	actorID := ""
	actorNickname := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorNickname = strings.TrimSpace(event.Actor.Nickname)
	}

	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}

	return outbound.BuildTargetLabel(ctx, targetType, targetID, targetName, actorID, actorNickname, resolver)
}
