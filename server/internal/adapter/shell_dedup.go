package adapter

import (
	"strings"
	"time"
)

func (s *Shell) isDuplicateEvent(eventID string, observedAt time.Time) bool {
	if strings.TrimSpace(eventID) == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := observedAt.Add(-recentEventDedupRetention)
	for key, seenAt := range s.recentEventIDs {
		if seenAt.Before(cutoff) {
			delete(s.recentEventIDs, key)
		}
	}
	if _, ok := s.recentEventIDs[eventID]; ok {
		s.dedupDrops++
		if s.metrics != nil {
			s.metrics.IncAdapterDedupDrop()
			s.metrics.IncEventPipelineStage("adapter", "dedup_drop")
		}
		return true
	}
	s.recentEventIDs[eventID] = observedAt
	if s.metrics != nil {
		s.metrics.IncEventPipelineStage("adapter", "accepted")
	}
	return false
}

// DedupDropsSnapshot returns the cumulative number of inbound events dropped
// because their event id matched a recently observed event within the
// dedup retention window. The counter is monotonically non-decreasing and
// safe to read from the bridge observability path.
func (s *Shell) DedupDropsSnapshot() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dedupDrops
}
