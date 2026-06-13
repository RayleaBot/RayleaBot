package bilibili

import "context"

func (s *Source) initializedDynamicUIDs(ctx context.Context, subjects map[string]Subject) map[string]bool {
	result := make(map[string]bool, len(subjects))
	for uid := range subjects {
		result[uid] = s.hasSeenDynamic(ctx, uid)
	}
	return result
}

func (s *Source) ensureDynamicBaselines(ctx context.Context, subjects map[string]Subject) {
	for uid := range subjects {
		key := EventDynamicPublished + ":baseline:" + uid
		s.markSeen(ctx, key, uid, EventDynamicPublished, "__baseline__")
	}
}

func (s *Source) hasSeenDynamic(ctx context.Context, uid string) bool {
	var exists int
	err := s.read.QueryRowContext(ctx,
		`SELECT 1 FROM bilibili_source_seen WHERE uid = ? AND event_type = ? LIMIT 1`,
		uid, EventDynamicPublished,
	).Scan(&exists)
	return err == nil && exists == 1
}
