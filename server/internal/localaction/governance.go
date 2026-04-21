package localaction

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
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

func (s *Service) executeGovernanceCommandPolicyRead(ctx context.Context, pluginID string) (map[string]any, error) {
	if err := s.requireGovernanceCapability(ctx, pluginID, "governance.command_policy.read"); err != nil {
		return nil, err
	}

	response, err := s.governance.ReadCommandPolicy(ctx)
	if err != nil {
		return nil, mapGovernanceRuntimeError("governance.command_policy.read failed", err)
	}
	return map[string]any{
		"default_level": response.DefaultLevel,
		"cooldown":      response.Cooldown,
		"commands":      response.Commands,
	}, nil
}

func (s *Service) requireGovernanceCapability(ctx context.Context, pluginID, capability string) error {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, capability) {
		return &runtime.Error{
			Code:    "permission.scope_violation",
			Message: capability + " capability is not granted",
		}
	}
	if s.governance == nil {
		return &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "governance service is not available",
		}
	}
	return nil
}

func mapGovernanceRuntimeError(message string, err error) error {
	switch {
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		return &runtime.Error{
			Code:    "platform.resource_missing",
			Message: message,
			Err:     err,
		}
	case errors.Is(err, governance.ErrInvalidRequest):
		return &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: message,
			Err:     err,
		}
	default:
		return &runtime.Error{
			Code:    "plugin.internal_error",
			Message: message,
			Err:     err,
		}
	}
}
