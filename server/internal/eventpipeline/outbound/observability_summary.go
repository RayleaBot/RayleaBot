package outbound

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

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

func summarizePlainText(plainText string) string {
	plainText = strings.TrimSpace(plainText)
	if plainText == "" {
		return "[空消息]"
	}
	return textsafe.TruncateRunes(plainText, 72, "...")
}
