package bilibili

import (
	"fmt"
	"html"
	"strings"
)

func dynamicRichTextSpan(className string, text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return fmt.Sprintf(`<span class="%s">%s</span>`, html.EscapeString(className), dynamicHTMLText(text))
}

func dynamicCSSClassToken(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r == '_' || r == '-':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func dynamicHTMLText(value any) string {
	text := dynamicRawText(value)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	escaped := html.EscapeString(text)
	return strings.ReplaceAll(escaped, "\n", "<br>")
}
