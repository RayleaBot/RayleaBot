package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func (s *logService) Replay(ctx context.Context) []logging.Summary {
	if s == nil {
		return nil
	}
	limit := 32
	if s.stream != nil && s.stream.Limit() > 0 {
		limit = s.stream.Limit()
	}
	items, err := s.listLogSummaries(ctx, logging.Query{
		BootID: s.CurrentBootID(),
		Limit:  limit,
	})
	if err != nil {
		return nil
	}
	return items
}

func (s *logService) Snapshot() []logging.Summary {
	if s == nil || s.stream == nil {
		return nil
	}
	return s.stream.Snapshot()
}

func (s *logService) Subscribe(buffer int) (<-chan logging.Summary, func()) {
	if s == nil || s.stream == nil {
		ch := make(chan logging.Summary)
		close(ch)
		return ch, func() {}
	}
	return s.stream.Subscribe(buffer)
}
