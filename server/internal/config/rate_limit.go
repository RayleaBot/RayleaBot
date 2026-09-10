package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type RateLimit struct {
	Count  int
	Window time.Duration
}

func ParseRateLimit(raw string) (RateLimit, error) {
	raw = strings.TrimSpace(raw)
	countText, windowText, ok := strings.Cut(raw, "/")
	if !ok {
		return RateLimit{}, fmt.Errorf("invalid rate limit format %q", raw)
	}

	count, err := strconv.Atoi(strings.TrimSpace(countText))
	if err != nil || count <= 0 {
		return RateLimit{}, fmt.Errorf("invalid rate limit count %q", countText)
	}

	window, err := time.ParseDuration(strings.TrimSpace(windowText))
	if err != nil || window <= 0 {
		return RateLimit{}, fmt.Errorf("invalid rate limit window %q", windowText)
	}

	return RateLimit{
		Count:  count,
		Window: window,
	}, nil
}
