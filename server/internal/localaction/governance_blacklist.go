package localaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) executeGovernanceBlacklistRead(ctx context.Context, pluginID string) (map[string]any, error) {
	if err := s.requireGovernanceCapability(ctx, pluginID, "governance.blacklist.read"); err != nil {
		return nil, err
	}

	snapshot, err := s.governance.ReadBlacklist(ctx)
	if err != nil {
		return nil, mapGovernanceRuntimeError("governance.blacklist.read failed", err)
	}
	return map[string]any{
		"user_entries":  snapshot.UserEntries,
		"group_entries": snapshot.GroupEntries,
	}, nil
}

func (s *Service) executeGovernanceBlacklistWrite(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if err := s.requireGovernanceCapability(ctx, pluginID, "governance.blacklist.write"); err != nil {
		return nil, err
	}

	switch action.GovernanceOperation {
	case "upsert":
		entry, err := s.governance.UpsertBlacklistEntry(ctx, action.GovernanceEntryType, action.GovernanceTargetID, action.GovernanceReason)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{
			"entry_type": entry.EntryType,
			"target_id":  entry.TargetID,
			"reason":     entry.Reason,
			"created_at": entry.CreatedAt,
		}, nil
	case "delete":
		if err := s.governance.DeleteBlacklistEntry(ctx, action.GovernanceEntryType, action.GovernanceTargetID); err != nil {
			return nil, mapGovernanceRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{
			"deleted": true,
		}, nil
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "governance.blacklist.write uses unsupported operation",
		}
	}
}
