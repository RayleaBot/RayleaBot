package source

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

func (s *Source) Status(ctx context.Context) Status {
	if s == nil {
		now := time.Now().UTC()
		return Status{
			Status:    StateDisabled,
			Summary:   sourceSummary(StateDisabled),
			Diagnosis: diagnosisForStatusAt(Status{Status: StateDisabled, Summary: sourceSummary(StateDisabled)}, nil, now),
		}
	}
	return s.statusWithAccounts(ctx)
}
func (s *Source) statusWithAccounts(ctx context.Context) Status {
	s.mu.RLock()
	status := s.status
	cooldowns := s.activeCooldownsLocked()
	s.mu.RUnlock()
	status.Status = normalizeSourceState(status.Status)
	status.Summary = sourceSummary(status.Status)
	return s.withAccountsAndDiagnosis(ctx, status, cooldowns)
}
func (s *Source) withAccounts(ctx context.Context, status Status) Status {
	accounts, err := s.accounts.List(ctx)
	if err == nil {
		status.Accounts = accounts
	}
	return status
}
func (s *Source) withAccountsAndDiagnosis(ctx context.Context, status Status, cooldowns []requestCooldown) Status {
	status = s.withAccounts(ctx, status)
	status.Diagnosis = s.diagnosisForStatus(status, cooldowns)
	return status
}
func (s *Source) publishStatus(ctx context.Context, status Status) {
	s.mu.RLock()
	cooldowns := s.activeCooldownsLocked()
	s.mu.RUnlock()
	status = s.withAccountsAndDiagnosis(ctx, status, cooldowns)
	if s.notifyStatus != nil {
		s.notifyStatus(status)
	}
	_ = s.persistStatus(ctx, status)
}
func (s *Source) persistStatus(ctx context.Context, status Status) error {
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	_, err = s.write.ExecContext(ctx,
		`INSERT INTO bilibili_source_state (key, value_json, updated_at)
		 VALUES ('status', ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`,
		string(raw), s.now().Format(time.RFC3339),
	)
	return err
}
func (s *Source) setLiveError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.status.Live.LastError = err.Error()
	s.status.Status = StateDegraded
	s.status.Summary = sourceSummary(StateDegraded)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}
func (s *Source) setDynamicError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.status.Dynamic.LastError = err.Error()
	s.status.Status = StateDegraded
	s.status.Summary = sourceSummary(StateDegraded)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}
func (s *Source) clearLiveError() {
	s.mu.Lock()
	s.status.Live.LastError = ""
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}
func (s *Source) deriveStateLocked() string {
	if s.status.Live.WatchedRooms == 0 && s.status.Dynamic.WatchedUIDs == 0 {
		return StateIdle
	}
	if s.status.Live.FailedRooms > 0 || s.status.Live.LastError != "" || s.status.Dynamic.LastError != "" {
		return StateDegraded
	}
	if s.status.Live.ConnectedRooms > 0 || s.status.Dynamic.LastPollAt != nil {
		return StateConnected
	}
	return StateConnecting
}
func (s *Source) diagnosisForStatus(status Status, cooldowns []requestCooldown) Diagnosis {
	return diagnosisForStatusAt(status, cooldowns, s.now())
}
func (s *Source) diagnosisForStatusLocked(status Status, cooldowns []requestCooldown) Diagnosis {
	if cooldowns == nil {
		cooldowns = s.activeCooldownsLocked()
	}
	return diagnosisForStatusAt(status, cooldowns, s.now())
}
func (s *Source) activeCooldownsLocked() []requestCooldown {
	now := s.now()
	items := make([]requestCooldown, 0, len(s.cooldowns))
	for _, cooldown := range s.cooldowns {
		if cooldown.Until.IsZero() || !now.Before(cooldown.Until) {
			continue
		}
		items = append(items, cooldown)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Scope == items[j].Scope {
			return items[i].Until.Before(items[j].Until)
		}
		return items[i].Scope < items[j].Scope
	})
	return items
}
