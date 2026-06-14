package dynamic

import "strings"

func classifyDynamicRichText(text string, node map[string]any) string {
	if dynamicRichTextEmojiURL(node) != "" {
		return "RICH_TEXT_NODE_TYPE_EMOJI"
	}
	if strings.HasPrefix(text, "#") && strings.HasSuffix(text, "#") && len([]rune(text)) > 2 {
		return "RICH_TEXT_NODE_TYPE_TOPIC"
	}
	if strings.HasPrefix(text, "@") {
		return "RICH_TEXT_NODE_TYPE_AT"
	}
	if text == "互动抽奖" {
		return "RICH_TEXT_NODE_TYPE_LOTTERY"
	}
	if strings.HasPrefix(text, "BV") && len(text) >= 10 {
		return "RICH_TEXT_NODE_TYPE_BV"
	}
	iconName := strings.ToLower(stringValue(node["icon_name"]))
	jumpURL := strings.ToLower(firstNonEmpty(stringValue(node["jump_url"]), stringValue(node["url"])))
	if strings.Contains(iconName, "vote") || strings.Contains(jumpURL, "vote") {
		return "RICH_TEXT_NODE_TYPE_VOTE"
	}
	if strings.Contains(iconName, "taobao") || strings.Contains(iconName, "goods") || strings.Contains(jumpURL, "mall") {
		return "RICH_TEXT_NODE_TYPE_GOODS"
	}
	return "RICH_TEXT_NODE_TYPE_TEXT"
}
