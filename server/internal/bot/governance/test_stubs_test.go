package governance

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil/permissiontest"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
)

// stubEntryRepo is an in-memory access list. The blacklist and whitelist
// repositories share the same contract, so one double serves both.
type stubEntryRepo struct {
	entries map[string]map[string]permission.Entry
}

func newStubEntryRepo() *stubEntryRepo {
	return &stubEntryRepo{entries: make(map[string]map[string]permission.Entry)}
}

func (s *stubEntryRepo) Contains(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), scope, entryType, targetID)
	return err == nil, nil
}

func (s *stubEntryRepo) Get(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (permission.Entry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.Entry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubEntryRepo) Add(_ context.Context, scope chatevent.IdentityScope, entryType, targetID, reason string) error {
	if s.entries[entryType] == nil {
		s.entries[entryType] = make(map[string]permission.Entry)
	}
	s.entries[entryType][targetID] = permission.Entry{
		Scope:     scope,
		EntryType: entryType,
		TargetID:  targetID,
		Reason:    reason,
		CreatedAt: "2026-04-20T00:00:00Z",
	}
	return nil
}

func (s *stubEntryRepo) Remove(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubEntryRepo) List(_ context.Context, entryType string) ([]permission.Entry, error) {
	items := make([]permission.Entry, 0, len(s.entries[entryType]))
	for _, entry := range s.entries[entryType] {
		items = append(items, entry)
	}
	return items, nil
}

type stubWhitelistStateRepo struct {
	enabled bool
}

func (s *stubWhitelistStateRepo) Enabled(context.Context) (bool, error) {
	return s.enabled, nil
}

func (s *stubWhitelistStateRepo) SetEnabled(_ context.Context, enabled bool) error {
	s.enabled = enabled
	return nil
}

func (s *stubEntryRepo) Page(ctx context.Context, query pagination.Query, entryType string) (permission.EntryPage, error) {
	return permissiontest.Page(ctx, s.List, query, entryType)
}
