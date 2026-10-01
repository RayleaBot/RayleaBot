package messagestats

import (
	"context"
	"slices"
	"time"
)

const notificationInterval = 2 * time.Second

// Change invalidates the statistics view without duplicating its counters.
type Change struct {
	ChangedAt  time.Time
	AdapterIDs []string
}

// markChanged requires mu. Notification dirtiness is independent of flush batches.
func (s *Service) markChanged(id string) {
	if id != "" {
		s.changed[id] = struct{}{}
	}
}

func (s *Service) takeChanges() Change {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.timeNow()
	for id, state := range s.states {
		if !state.offlineNotified && !state.since.IsZero() && now.Sub(state.since) >= 30*time.Second {
			s.markChanged(id)
			state.offlineNotified = true
			s.states[id] = state
		}
	}
	change := Change{ChangedAt: now, AdapterIDs: make([]string, 0, len(s.changed))}
	for id := range s.changed {
		change.AdapterIDs = append(change.AdapterIDs, id)
	}
	clear(s.changed)
	return change
}

// The notifier is independent of database I/O so a slow flush cannot delay it.
// Its worker is joined by Stop before the final flush.
func (s *Service) runNotifier(ctx context.Context) {
	timer := time.NewTimer(notificationInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			change := s.takeChanges()
			if ctx.Err() != nil {
				return
			}
			if len(change.AdapterIDs) > 0 {
				slices.Sort(change.AdapterIDs)
				s.notifyChanged(change)
			}
			// Reset after publishing: delayed ticks must not produce a burst.
			timer.Reset(notificationInterval)
		}
	}
}
