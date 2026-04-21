package outbound

import (
	"context"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

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
	targetName = strings.TrimSpace(textsafe.SanitizeString(targetName))
	actorID = strings.TrimSpace(actorID)
	actorNickname = strings.TrimSpace(textsafe.SanitizeString(actorNickname))

	switch targetType {
	case "group":
		if targetName == "" && resolver != nil {
			targetName = strings.TrimSpace(textsafe.SanitizeString(resolver.ResolveTargetName(ctx, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, targetName)
	case "private":
		displayName := ""
		if actorID != "" && actorID == targetID {
			displayName = actorNickname
		}
		if displayName == "" && resolver != nil {
			displayName = strings.TrimSpace(textsafe.SanitizeString(resolver.ResolveTargetName(ctx, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, displayName)
	default:
		if targetName == "" && resolver != nil {
			targetName = strings.TrimSpace(textsafe.SanitizeString(resolver.ResolveTargetName(ctx, targetType, targetID)))
		}
		return formatTargetLabel(targetType, targetID, targetName)
	}
}

func formatTargetLabel(targetType string, targetID string, displayName string) string {
	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	displayName = strings.TrimSpace(textsafe.SanitizeString(displayName))

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
