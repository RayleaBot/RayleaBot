package logging

import (
	"context"
	"time"
)

const retentionSweepInterval = 10 * time.Minute

// RunRetentionLoop is owned by the App supervisor and stops before SQLite closes.
func (s *Stream) RunRetentionLoop(ctx context.Context) {
	ticker := time.NewTicker(retentionSweepInterval)
	defer ticker.Stop()
	for {
		if err := s.pruneExpired(ctx); err != nil && ctx.Err() == nil {
			s.reportPersistenceFailure("management log retention cleanup failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Stream) pruneExpired(ctx context.Context) error {
	s.mu.RLock()
	repository, days := s.repository, s.retentionDays
	s.mu.RUnlock()
	if repository == nil || days <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return repository.PruneOlderThan(ctx, s.now().AddDate(0, 0, -days))
}
