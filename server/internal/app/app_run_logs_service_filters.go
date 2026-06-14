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

func normalizeRepeatedQueryValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized
}
