package logging

import (
	"context"
	"strings"
	"time"
)

type ManagementService struct {
	stream     *Stream
	repository Repository
}

func NewManagementService(stream *Stream, repository Repository) *ManagementService {
	return &ManagementService{stream: stream, repository: repository}
}

func (s *ManagementService) SetRepository(repository Repository) {
	if s == nil {
		return
	}
	s.repository = repository
}

func (s *ManagementService) CurrentBootID() string {
	if s == nil || s.stream == nil {
		return ""
	}
	return s.stream.BootID()
}

func (s *ManagementService) GetLogSummary(ctx context.Context, logID string) (Summary, error) {
	trimmedLogID := strings.TrimSpace(logID)
	if s != nil && s.repository != nil {
		item, err := s.repository.GetSummary(ctx, trimmedLogID)
		if err == nil {
			return item, nil
		}
		if err != ErrLogNotFound {
			return Summary{}, err
		}
		if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
			return item, nil
		}
		return Summary{}, ErrLogNotFound
	}

	if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
		return item, nil
	}

	if s == nil || s.stream == nil {
		return Summary{}, ErrLogNotFound
	}

	return Summary{}, ErrLogNotFound
}

func (s *ManagementService) findStreamLogSummary(logID string) (Summary, bool) {
	if s == nil || s.stream == nil || logID == "" {
		return Summary{}, false
	}

	for _, item := range s.stream.Snapshot() {
		if item.LogID == logID {
			return item, true
		}
	}

	return Summary{}, false
}

func (s *ManagementService) listLogSummaries(ctx context.Context, query Query) ([]Summary, error) {
	if s != nil && s.repository != nil {
		return s.repository.ListSummaries(ctx, query)
	}

	items := make([]Summary, 0)
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

func (s *ManagementService) ListLogPage(ctx context.Context, query PageQuery) (PageResult, error) {
	if s != nil && s.repository != nil {
		return s.repository.ListPage(ctx, query)
	}

	items, err := s.listLogSummaries(ctx, Query{
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
		return PageResult{}, err
	}
	reversed := make([]Summary, 0, len(items))
	for index := len(items) - 1; index >= 0; index-- {
		reversed = append(reversed, items[index])
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	return PageResult{
		Items: reversed,
		Page: PageInfo{
			Limit: limit,
		},
	}, nil
}

func (s *ManagementService) Replay(ctx context.Context) []Summary {
	if s == nil {
		return nil
	}
	limit := 32
	if s.stream != nil && s.stream.Limit() > 0 {
		limit = s.stream.Limit()
	}
	items, err := s.listLogSummaries(ctx, Query{
		BootID: s.CurrentBootID(),
		Limit:  limit,
	})
	if err != nil {
		return nil
	}
	return items
}

func (s *ManagementService) Snapshot() []Summary {
	if s == nil || s.stream == nil {
		return nil
	}
	return s.stream.Snapshot()
}

func (s *ManagementService) Subscribe(buffer int) (<-chan Summary, func()) {
	if s == nil || s.stream == nil {
		ch := make(chan Summary)
		close(ch)
		return ch, func() {}
	}
	return s.stream.Subscribe(buffer)
}

func logSummaryMatchesTimeRange(summary Summary, startAt, endAt string) bool {
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
