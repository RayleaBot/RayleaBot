package app

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func logSummaryMatchesTimeRange(summary logging.Summary, startAt, endAt string) bool {
	if startAt == "" && endAt == "" {
		return true
	}

	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(summary.Timestamp))
	if err != nil {
		return false
	}
	normalized := timestamp.UTC()
	if startAt != "" {
		start, err := time.Parse(time.RFC3339, startAt)
		if err != nil || normalized.Before(start.UTC()) {
			return false
		}
	}
	if endAt != "" {
		end, err := time.Parse(time.RFC3339, endAt)
		if err != nil || normalized.After(end.UTC()) {
			return false
		}
	}
	return true
}

func matchesRepeatedLogFilter(value, single string, values []string) bool {
	filters := normalizeRepeatedQueryValues(append([]string{single}, values...))
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if value == filter {
			return true
		}
	}
	return false
}
