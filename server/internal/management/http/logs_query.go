package managementhttp

import (
	"errors"
	"strings"
	"time"
)

func parseLogScope(raw string) (logScope, error) {
	switch strings.TrimSpace(raw) {
	case "", string(logScopeHistory):
		return logScopeHistory, nil
	case string(logScopeCurrentSession):
		return logScopeCurrentSession, nil
	default:
		return "", errors.New("unsupported log scope")
	}
}

func parseLogTimeRange(scope logScope, rawStartAt, rawEndAt string) (string, string, error) {
	startAt := strings.TrimSpace(rawStartAt)
	endAt := strings.TrimSpace(rawEndAt)
	if scope == logScopeCurrentSession {
		if startAt != "" || endAt != "" {
			return "", "", errors.New("current session scope does not support time range")
		}
		return "", "", nil
	}

	startUTC, err := normalizeLogQueryTime(startAt)
	if err != nil {
		return "", "", err
	}
	endUTC, err := normalizeLogQueryTime(endAt)
	if err != nil {
		return "", "", err
	}
	if startUTC != "" && endUTC != "" && startUTC > endUTC {
		return "", "", errors.New("start_at is later than end_at")
	}
	return startUTC, endUTC, nil
}

func normalizeLogQueryTime(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return "", err
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func isAllowedLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
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
