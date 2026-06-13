package governance

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

func (s *Service) ReadWhitelist(ctx context.Context) (WhitelistSnapshot, error) {
	if s == nil {
		return WhitelistSnapshot{
			Enabled:      false,
			UserEntries:  []EntryResponse{},
			GroupEntries: []EntryResponse{},
		}, nil
	}

	enabled, err := whitelistEnabled(ctx, s.whitelistState)
	if err != nil {
		return WhitelistSnapshot{}, err
	}
	userEntries, groupEntries, err := whitelistEntries(ctx, s.whitelistRepo)
	if err != nil {
		return WhitelistSnapshot{}, err
	}

	return WhitelistSnapshot{
		Enabled:      enabled,
		UserEntries:  userEntries,
		GroupEntries: groupEntries,
	}, nil
}

func (s *Service) SetWhitelistEnabled(ctx context.Context, enabled bool) (WhitelistStateResponse, error) {
	if s == nil || s.whitelistState == nil {
		return WhitelistStateResponse{}, ErrServiceUnavailable
	}
	if err := s.whitelistState.SetEnabled(ctx, enabled); err != nil {
		return WhitelistStateResponse{}, err
	}
	s.notify(defaultGovernanceSummary)
	return WhitelistStateResponse{Enabled: enabled}, nil
}

func (s *Service) UpsertWhitelistEntry(ctx context.Context, entryType, targetID, reason string) (EntryResponse, error) {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	reason = strings.TrimSpace(reason)
	if !validEntryInput(entryType, targetID, reason) {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s == nil || s.whitelistRepo == nil {
		return EntryResponse{}, ErrServiceUnavailable
	}

	if err := s.whitelistRepo.Add(ctx, entryType, targetID, reason); err != nil {
		return EntryResponse{}, err
	}
	entry, err := s.whitelistRepo.Get(ctx, entryType, targetID)
	if err != nil {
		return EntryResponse{}, err
	}
	s.notify(defaultGovernanceSummary)
	return buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt), nil
}

func (s *Service) DeleteWhitelistEntry(ctx context.Context, entryType, targetID string) error {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	if !validEntryDeleteInput(entryType, targetID) {
		return ErrInvalidRequest
	}
	if s == nil || s.whitelistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.whitelistRepo.Remove(ctx, entryType, targetID); err != nil {
		return err
	}
	s.notify(defaultGovernanceSummary)
	return nil
}

func whitelistEnabled(ctx context.Context, repo permission.WhitelistStateRepository) (bool, error) {
	if repo == nil {
		return false, nil
	}
	return repo.Enabled(ctx)
}

func whitelistEntries(ctx context.Context, repo permission.WhitelistRepository) ([]EntryResponse, []EntryResponse, error) {
	if repo == nil {
		return []EntryResponse{}, []EntryResponse{}, nil
	}

	userEntries, err := repo.List(ctx, "user")
	if err != nil {
		return nil, nil, err
	}
	groupEntries, err := repo.List(ctx, "group")
	if err != nil {
		return nil, nil, err
	}
	return buildWhitelistEntries(userEntries), buildWhitelistEntries(groupEntries), nil
}

func buildWhitelistEntries(entries []permission.WhitelistEntry) []EntryResponse {
	if len(entries) == 0 {
		return []EntryResponse{}
	}

	items := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
	return items
}
