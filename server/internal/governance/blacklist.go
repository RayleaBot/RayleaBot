package governance

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

func (s *Service) ReadBlacklist(ctx context.Context) (BlacklistSnapshot, error) {
	if s == nil || s.blacklistRepo == nil {
		return BlacklistSnapshot{
			UserEntries:  []EntryResponse{},
			GroupEntries: []EntryResponse{},
		}, nil
	}

	userEntries, err := s.blacklistRepo.List(ctx, "user")
	if err != nil {
		return BlacklistSnapshot{}, err
	}
	groupEntries, err := s.blacklistRepo.List(ctx, "group")
	if err != nil {
		return BlacklistSnapshot{}, err
	}

	return BlacklistSnapshot{
		UserEntries:  buildBlacklistEntries(userEntries),
		GroupEntries: buildBlacklistEntries(groupEntries),
	}, nil
}

func (s *Service) UpsertBlacklistEntry(ctx context.Context, entryType, targetID, reason string) (EntryResponse, error) {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	reason = strings.TrimSpace(reason)
	if !validEntryInput(entryType, targetID, reason) {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s == nil || s.blacklistRepo == nil {
		return EntryResponse{}, ErrServiceUnavailable
	}

	if err := s.blacklistRepo.Add(ctx, entryType, targetID, reason); err != nil {
		return EntryResponse{}, err
	}
	entry, err := s.blacklistRepo.Get(ctx, entryType, targetID)
	if err != nil {
		return EntryResponse{}, err
	}
	s.notify(defaultGovernanceSummary)
	return buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt), nil
}

func (s *Service) DeleteBlacklistEntry(ctx context.Context, entryType, targetID string) error {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	if !validEntryDeleteInput(entryType, targetID) {
		return ErrInvalidRequest
	}
	if s == nil || s.blacklistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.blacklistRepo.Remove(ctx, entryType, targetID); err != nil {
		return err
	}
	s.notify(defaultGovernanceSummary)
	return nil
}

func buildBlacklistEntries(entries []permission.BlacklistEntry) []EntryResponse {
	if len(entries) == 0 {
		return []EntryResponse{}
	}

	items := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
	return items
}
