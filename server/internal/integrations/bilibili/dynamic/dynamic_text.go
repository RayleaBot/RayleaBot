package dynamic

import (
	"strings"
)

func dynamicText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case map[string]any:
		for _, key := range []string{"text", "orig_text", "title", "desc", "summary", "content"} {
			if text := dynamicText(typed[key]); text != "" {
				return text
			}
		}
		for _, key := range []string{"rich_text_nodes", "paragraphs"} {
			if text := dynamicText(typed[key]); text != "" {
				return text
			}
		}
		return ""
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := dynamicText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, ""))
	default:
		return normalizeDynamicPlainText(dynamicRawText(typed))
	}
}
func dynamicRawText(value any) string {
	text := stringValue(value)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\\r\\n", "\n")
	text = strings.ReplaceAll(text, "\\n", "\n")
	text = strings.ReplaceAll(text, "\\t", " ")
	return text
}
func normalizeDynamicPlainText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		if cleaned := strings.Join(strings.Fields(line), " "); cleaned != "" {
			normalized = append(normalized, cleaned)
		}
	}
	return strings.Join(normalized, "\n")
}
