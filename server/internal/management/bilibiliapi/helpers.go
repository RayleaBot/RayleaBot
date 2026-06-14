package bilibiliapi

import "time"

func isDigits(raw string) bool {
	if raw == "" {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func timeStringPtr(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	text := value.UTC().Format(time.RFC3339)
	return &text
}
