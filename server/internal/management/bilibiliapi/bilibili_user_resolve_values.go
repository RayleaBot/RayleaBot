package bilibiliapi

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var bilibiliHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)

func cleanBilibiliUserText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	text = bilibiliHTMLTagPattern.ReplaceAllString(text, "")
	return strings.TrimSpace(html.UnescapeString(text))
}

func bilibiliIDText(value any) string {
	switch typed := value.(type) {
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return cleanBilibiliUserText(value)
	}
}

func cleanBilibiliUserURL(value any) string {
	text := cleanBilibiliUserText(value)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "//") {
		return "https:" + text
	}
	if strings.Contains(text, "://") {
		return text
	}
	return ""
}

func firstNonEmpty(values ...any) any {
	for _, value := range values {
		if cleanBilibiliUserText(value) != "" {
			return value
		}
	}
	return nil
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		number, _ := typed.Int64()
		return int(number)
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}
