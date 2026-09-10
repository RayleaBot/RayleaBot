package governance

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/pagination"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
)

type stubBlacklistRepo struct {
	entries map[string]map[string]permission.Entry
}

func newStubBlacklistRepo() *stubBlacklistRepo {
	return &stubBlacklistRepo{entries: make(map[string]map[string]permission.Entry)}
}

func (s *stubBlacklistRepo) Contains(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), scope, entryType, targetID)
	return err == nil, nil
}

func (s *stubBlacklistRepo) Get(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (permission.Entry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.Entry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubBlacklistRepo) Add(_ context.Context, scope chatevent.IdentityScope, entryType, targetID, reason string) error {
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

func (s *stubBlacklistRepo) Remove(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubBlacklistRepo) List(_ context.Context, entryType string) ([]permission.Entry, error) {
	items := make([]permission.Entry, 0, len(s.entries[entryType]))
	for _, entry := range s.entries[entryType] {
		items = append(items, entry)
	}
	return items, nil
}

type stubWhitelistRepo struct {
	entries map[string]map[string]permission.Entry
}

func newStubWhitelistRepo() *stubWhitelistRepo {
	return &stubWhitelistRepo{entries: make(map[string]map[string]permission.Entry)}
}

func (s *stubWhitelistRepo) Contains(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), scope, entryType, targetID)
	return err == nil, nil
}

func (s *stubWhitelistRepo) Get(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (permission.Entry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.Entry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubWhitelistRepo) Add(_ context.Context, scope chatevent.IdentityScope, entryType, targetID, reason string) error {
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

func (s *stubWhitelistRepo) Remove(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubWhitelistRepo) List(_ context.Context, entryType string) ([]permission.Entry, error) {
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

func (s *stubBlacklistRepo) Page(ctx context.Context, query pagination.Query, entryType string) (permission.EntryPage, error) {
	users, err := s.List(ctx, "user")
	if err != nil {
		return permission.EntryPage{}, err
	}
	groups, err := s.List(ctx, "group")
	if err != nil {
		return permission.EntryPage{}, err
	}
	all := append(users, groups...)
	filtered := make([]permission.Entry, 0, len(all))
	for _, item := range all {
		if (entryType == "" || item.EntryType == entryType) && pagination.Matches(query.Text, item.TargetID, item.Reason) {
			filtered = append(filtered, item)
		}
	}
	items, meta := pagination.Slice(filtered, query)
	return permission.EntryPage{Items: items, Total: meta.Total, EntryCount: len(all)}, nil
}

func (s *stubWhitelistRepo) Page(ctx context.Context, query pagination.Query, entryType string) (permission.EntryPage, error) {
	users, err := s.List(ctx, "user")
	if err != nil {
		return permission.EntryPage{}, err
	}
	groups, err := s.List(ctx, "group")
	if err != nil {
		return permission.EntryPage{}, err
	}
	all := append(users, groups...)
	filtered := make([]permission.Entry, 0, len(all))
	for _, item := range all {
		if (entryType == "" || item.EntryType == entryType) && pagination.Matches(query.Text, item.TargetID, item.Reason) {
			filtered = append(filtered, item)
		}
	}
	items, meta := pagination.Slice(filtered, query)
	return permission.EntryPage{Items: items, Total: meta.Total, EntryCount: len(all)}, nil
}
