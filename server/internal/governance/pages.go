package governance

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

// ManagementEntryRepository keeps paging requirements out of the chat policy's
// membership-only collaborator while serving bounded management reads.
type ManagementEntryRepository interface {
	permission.EntryRepository
	Page(context.Context, pagination.Query, string) (permission.EntryPage, error)
}

type BlacklistPage struct {
	BlacklistSnapshot
	pagination.Metadata
	EntryCount int `json:"entry_count"`
}

type WhitelistPage struct {
	WhitelistSnapshot
	pagination.Metadata
	EntryCount int `json:"entry_count"`
}

func (s *Service) ReadBlacklistPage(ctx context.Context, query pagination.Query, entryType string) (BlacklistPage, error) {
	if entryType != "" && !IsEntryType(entryType) {
		return BlacklistPage{}, ErrInvalidRequest
	}
	if s.blacklistRepo == nil {
		return BlacklistPage{}, ErrServiceUnavailable
	}
	page, err := s.blacklistRepo.Page(ctx, query, entryType)
	if err != nil {
		return BlacklistPage{}, err
	}
	users, groups := splitPageEntries(page.Items)
	return BlacklistPage{BlacklistSnapshot: BlacklistSnapshot{UserEntries: users, GroupEntries: groups}, Metadata: pagination.Meta(query, page.Total), EntryCount: page.EntryCount}, nil
}

func (s *Service) ReadWhitelistPage(ctx context.Context, query pagination.Query, entryType string) (WhitelistPage, error) {
	if entryType != "" && !IsEntryType(entryType) {
		return WhitelistPage{}, ErrInvalidRequest
	}
	if s.whitelistRepo == nil {
		return WhitelistPage{}, ErrServiceUnavailable
	}
	enabled, err := whitelistEnabled(ctx, s.whitelistState)
	if err != nil {
		return WhitelistPage{}, err
	}
	page, err := s.whitelistRepo.Page(ctx, query, entryType)
	if err != nil {
		return WhitelistPage{}, err
	}
	users, groups := splitPageEntries(page.Items)
	return WhitelistPage{WhitelistSnapshot: WhitelistSnapshot{Enabled: enabled, UserEntries: users, GroupEntries: groups}, Metadata: pagination.Meta(query, page.Total), EntryCount: page.EntryCount}, nil
}

func splitPageEntries(entries []permission.Entry) ([]EntryResponse, []EntryResponse) {
	users, groups := []EntryResponse{}, []EntryResponse{}
	for _, entry := range entries {
		if entry.EntryType == "user" {
			users = append(users, buildEntryResponse(entry))
		} else {
			groups = append(groups, buildEntryResponse(entry))
		}
	}
	return users, groups
}
