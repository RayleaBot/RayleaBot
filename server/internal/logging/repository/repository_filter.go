package repository

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

type filterSpec struct {
	Level     string
	Levels    []string
	Source    string
	Protocol  string
	PluginID  string
	PluginIDs []string
	RequestID string
	BootID    string
	StartAt   string
	EndAt     string
}

const logTimestampExpr = "julianday(ts)"

func buildLogFilterClauses(spec filterSpec) ([]string, []any, error) {
	clauses := []string{"1 = 1"}
	args := make([]any, 0, 8)
	if levels := normalizeFilterValues(spec.Level, spec.Levels, true); len(levels) > 0 {
		clauses, args = appendStringSetClause(clauses, args, "level", levels)
	}
	if spec.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, strings.TrimSpace(spec.Source))
	}
	if spec.Protocol != "" {
		sources := logging.SourcesForProtocol(spec.Protocol)
		if len(sources) == 0 {
			return []string{"1 = 0"}, args, nil
		}
		placeholders := make([]string, 0, len(sources))
		for _, source := range sources {
			placeholders = append(placeholders, "?")
			args = append(args, source)
		}
		clauses = append(clauses, "source IN ("+strings.Join(placeholders, ", ")+")")
	}
	if pluginIDs := normalizeFilterValues(spec.PluginID, spec.PluginIDs, false); len(pluginIDs) > 0 {
		clauses, args = appendStringSetClause(clauses, args, "plugin_id", pluginIDs)
	}
	if spec.RequestID != "" {
		clauses = append(clauses, "request_id = ?")
		args = append(args, strings.TrimSpace(spec.RequestID))
	}
	if spec.BootID != "" {
		clauses = append(clauses, "boot_id = ?")
		args = append(args, strings.TrimSpace(spec.BootID))
	}
	if spec.StartAt != "" {
		clauses = append(clauses, logTimestampExpr+" >= julianday(?)")
		args = append(args, strings.TrimSpace(spec.StartAt))
	}
	if spec.EndAt != "" {
		clauses = append(clauses, logTimestampExpr+" <= julianday(?)")
		args = append(args, strings.TrimSpace(spec.EndAt))
	}
	return clauses, args, nil
}

func normalizeFilterValues(single string, values []string, lower bool) []string {
	normalized := make([]string, 0, len(values)+1)
	seen := make(map[string]struct{}, len(values)+1)
	for _, value := range append([]string{single}, values...) {
		item := strings.TrimSpace(value)
		if lower {
			item = strings.ToLower(item)
		}
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

func appendStringSetClause(clauses []string, args []any, column string, values []string) ([]string, []any) {
	if len(values) == 1 {
		clauses = append(clauses, column+" = ?")
		args = append(args, values[0])
		return clauses, args
	}

	placeholders := make([]string, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	clauses = append(clauses, column+" IN ("+strings.Join(placeholders, ", ")+")")
	return clauses, args
}
