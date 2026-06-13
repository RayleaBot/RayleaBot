package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func (s *logService) listLogSummaries(ctx context.Context, query logging.Query) ([]logging.Summary, error) {
	if s != nil && s.repository != nil {
		return s.repository.ListSummaries(ctx, query)
	}

	items := make([]logging.Summary, 0)
	if s == nil || s.stream == nil {
		return items, nil
	}
	for _, summary := range s.stream.Snapshot() {
		if query.BootID != "" && summary.BootID != query.BootID {
			continue
		}
		if !matchesRepeatedLogFilter(summary.Level, query.Level, query.Levels) {
			continue
		}
		if query.Source != "" && summary.Source != query.Source {
			continue
		}
		if query.Protocol != "" && summary.Protocol != query.Protocol {
			continue
		}
		if !matchesRepeatedLogFilter(summary.PluginID, query.PluginID, query.PluginIDs) {
			continue
		}
		if query.RequestID != "" && summary.RequestID != query.RequestID {
			continue
		}
		if !logSummaryMatchesTimeRange(summary, query.StartAt, query.EndAt) {
			continue
		}
		items = append(items, summary)
	}
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[len(items)-query.Limit:]
	}
	return items, nil
}

func (s *logService) listLogPage(ctx context.Context, query logging.PageQuery) (logging.PageResult, error) {
	if s != nil && s.repository != nil {
		return s.repository.ListPage(ctx, query)
	}

	items, err := s.listLogSummaries(ctx, logging.Query{
		Level:     query.Level,
		Levels:    query.Levels,
		Source:    query.Source,
		Protocol:  query.Protocol,
		PluginID:  query.PluginID,
		PluginIDs: query.PluginIDs,
		RequestID: query.RequestID,
		BootID:    query.BootID,
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
		Limit:     query.Limit,
	})
	if err != nil {
		return logging.PageResult{}, err
	}
	reversed := make([]logging.Summary, 0, len(items))
	for index := len(items) - 1; index >= 0; index-- {
		reversed = append(reversed, items[index])
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	return logging.PageResult{
		Items: reversed,
		Page: logging.PageInfo{
			Limit: limit,
		},
	}, nil
}
