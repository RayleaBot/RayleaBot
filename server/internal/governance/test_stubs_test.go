package governance

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

type stubBlacklistRepo struct {
	entries map[string]map[string]permission.BlacklistEntry
}

func newStubBlacklistRepo() *stubBlacklistRepo {
	return &stubBlacklistRepo{entries: make(map[string]map[string]permission.BlacklistEntry)}
}

func (s *stubBlacklistRepo) IsBlacklisted(_ context.Context, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), entryType, targetID)
	return err == nil, nil
}

func (s *stubBlacklistRepo) Get(_ context.Context, entryType, targetID string) (permission.BlacklistEntry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.BlacklistEntry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubBlacklistRepo) Add(_ context.Context, entryType, targetID, reason string) error {
	if s.entries[entryType] == nil {
		s.entries[entryType] = make(map[string]permission.BlacklistEntry)
	}
	s.entries[entryType][targetID] = permission.BlacklistEntry{
		EntryType: entryType,
		TargetID:  targetID,
		Reason:    reason,
		CreatedAt: "2026-04-20T00:00:00Z",
	}
	return nil
}

func (s *stubBlacklistRepo) Remove(_ context.Context, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubBlacklistRepo) List(_ context.Context, entryType string) ([]permission.BlacklistEntry, error) {
	items := make([]permission.BlacklistEntry, 0, len(s.entries[entryType]))
	for _, entry := range s.entries[entryType] {
		items = append(items, entry)
	}
	return items, nil
}

type stubWhitelistRepo struct {
	entries map[string]map[string]permission.WhitelistEntry
}

func newStubWhitelistRepo() *stubWhitelistRepo {
	return &stubWhitelistRepo{entries: make(map[string]map[string]permission.WhitelistEntry)}
}

func (s *stubWhitelistRepo) IsWhitelisted(_ context.Context, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), entryType, targetID)
	return err == nil, nil
}

func (s *stubWhitelistRepo) Get(_ context.Context, entryType, targetID string) (permission.WhitelistEntry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.WhitelistEntry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubWhitelistRepo) Add(_ context.Context, entryType, targetID, reason string) error {
	if s.entries[entryType] == nil {
		s.entries[entryType] = make(map[string]permission.WhitelistEntry)
	}
	s.entries[entryType][targetID] = permission.WhitelistEntry{
		EntryType: entryType,
		TargetID:  targetID,
		Reason:    reason,
		CreatedAt: "2026-04-20T00:00:00Z",
	}
	return nil
}

func (s *stubWhitelistRepo) Remove(_ context.Context, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubWhitelistRepo) List(_ context.Context, entryType string) ([]permission.WhitelistEntry, error) {
	items := make([]permission.WhitelistEntry, 0, len(s.entries[entryType]))
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
