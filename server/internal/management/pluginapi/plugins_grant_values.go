package pluginapi

import (
	"errors"
	"strings"
	"time"
)

func parseGrantRequestExpiry(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	raw := strings.TrimSpace(*value)
	if raw == "" || !strings.HasSuffix(raw, "Z") {
		return nil, errors.New("expires_at must be a UTC RFC3339 timestamp")
	}

	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	if !parsed.After(time.Now().UTC()) {
		return nil, errors.New("expires_at must be in the future")
	}
	return &parsed, nil
}
