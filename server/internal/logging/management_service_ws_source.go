package logging

import "context"

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
