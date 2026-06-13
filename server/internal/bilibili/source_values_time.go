package bilibili

import (
	"strings"
	"time"
)

func formatCooldownDelay(delay time.Duration) string {
	if delay <= 0 {
		return "0s"
	}
	return delay.Round(time.Second).String()
}

func nullableTimeString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func parseRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseRFC3339Ptr(value string) *time.Time {
	parsed := parseRFC3339(value)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func formatTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}
