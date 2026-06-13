package localaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) executeGovernanceWhitelistRead(ctx context.Context, pluginID string) (map[string]any, error) {
	if err := s.requireGovernanceCapability(ctx, pluginID, "governance.whitelist.read"); err != nil {
		return nil, err
	}

	snapshot, err := s.governance.ReadWhitelist(ctx)
	if err != nil {
		return nil, mapGovernanceRuntimeError("governance.whitelist.read failed", err)
	}
	return map[string]any{
		"enabled":       snapshot.Enabled,
		"user_entries":  snapshot.UserEntries,
		"group_entries": snapshot.GroupEntries,
	}, nil
}

func (s *Service) executeGovernanceWhitelistWrite(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if err := s.requireGovernanceCapability(ctx, pluginID, "governance.whitelist.write"); err != nil {
		return nil, err
	}

	switch action.GovernanceOperation {
	case "set_enabled":
		if action.GovernanceEnabled == nil {
			return nil, &runtime.Error{
				Code:    "plugin.protocol_violation",
				Message: "governance.whitelist.write is missing enabled",
			}
		}
		response, err := s.governance.SetWhitelistEnabled(ctx, *action.GovernanceEnabled)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{
			"enabled": response.Enabled,
		}, nil
	case "upsert":
		entry, err := s.governance.UpsertWhitelistEntry(ctx, action.GovernanceEntryType, action.GovernanceTargetID, action.GovernanceReason)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{
			"entry_type": entry.EntryType,
			"target_id":  entry.TargetID,
			"reason":     entry.Reason,
			"created_at": entry.CreatedAt,
		}, nil
	case "delete":
		if err := s.governance.DeleteWhitelistEntry(ctx, action.GovernanceEntryType, action.GovernanceTargetID); err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{
			"deleted": true,
		}, nil
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "governance.whitelist.write uses unsupported operation",
		}
	}
}
