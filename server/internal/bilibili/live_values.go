package bilibili

import (
	"strconv"
	"strings"
	"time"
)

func liveImages(item liveStatusItem) []Image {
	if url := firstLiveImageURL(item); url != "" {
		return []Image{{URL: url}}
	}
	return nil
}

func firstLiveImageURL(item liveStatusItem) string {
	for _, value := range []string{item.CoverFromUser, item.UserCover} {
		if url := normalizeURL(value); url != "" {
			return url
		}
	}
	return ""
}

func liveTimeFromItem(item liveStatusItem) int64 {
	for _, value := range []any{item.LiveTime, item.LiveTimeCompat} {
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return int64(typed)
			}
		case int:
			if typed > 0 {
				return int64(typed)
			}
		case int64:
			if typed > 0 {
				return typed
			}
		case string:
			text := strings.TrimSpace(typed)
			if text == "" || text == "0000-00-00 00:00:00" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil && parsed > 0 {
				return parsed
			}
			if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.Local); err == nil {
				return parsed.Unix()
			}
		}
	}
	return 0
}

func normalizeLiveStatus(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func parseInt(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}
