package governanceaction

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
}

type Service interface {
	ReadBlacklist(context.Context) (governance.BlacklistSnapshot, error)
	UpsertBlacklistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteBlacklistEntry(context.Context, string, string) error
	ReadWhitelist(context.Context) (governance.WhitelistSnapshot, error)
	SetWhitelistEnabled(context.Context, bool) (governance.WhitelistStateResponse, error)
	UpsertWhitelistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteWhitelistEntry(context.Context, string, string) error
	ReadCommandPolicy(context.Context) (governance.CommandPolicyResponse, error)
}

type Request struct {
	PluginID     string
	Action       runtimeaction.Action
	Capabilities CapabilityView
	Service      Service
}

func ExecuteBlacklistRead(ctx context.Context, req Request) (map[string]any, error) {
	if err := requireCapability(ctx, req, "governance.blacklist.read"); err != nil {
		return nil, err
	}

	snapshot, err := req.Service.ReadBlacklist(ctx)
	if err != nil {
		return nil, mapRuntimeError("governance.blacklist.read failed", err)
	}
	return map[string]any{
		"user_entries":  snapshot.UserEntries,
		"group_entries": snapshot.GroupEntries,
	}, nil
}

func ExecuteBlacklistWrite(ctx context.Context, req Request) (map[string]any, error) {
	if err := requireCapability(ctx, req, "governance.blacklist.write"); err != nil {
		return nil, err
	}

	switch req.Action.GovernanceOperation {
	case "upsert":
		entry, err := req.Service.UpsertBlacklistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{
			"entry_type": entry.EntryType,
			"target_id":  entry.TargetID,
			"reason":     entry.Reason,
			"created_at": entry.CreatedAt,
		}, nil
	case "delete":
		if err := req.Service.DeleteBlacklistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{
			"deleted": true,
		}, nil
	default:
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "governance.blacklist.write uses unsupported operation",
		}
	}
}

func ExecuteWhitelistRead(ctx context.Context, req Request) (map[string]any, error) {
	if err := requireCapability(ctx, req, "governance.whitelist.read"); err != nil {
		return nil, err
	}

	snapshot, err := req.Service.ReadWhitelist(ctx)
	if err != nil {
		return nil, mapRuntimeError("governance.whitelist.read failed", err)
	}
	return map[string]any{
		"enabled":       snapshot.Enabled,
		"user_entries":  snapshot.UserEntries,
		"group_entries": snapshot.GroupEntries,
	}, nil
}

func ExecuteWhitelistWrite(ctx context.Context, req Request) (map[string]any, error) {
	if err := requireCapability(ctx, req, "governance.whitelist.write"); err != nil {
		return nil, err
	}

	switch req.Action.GovernanceOperation {
	case "set_enabled":
		if req.Action.GovernanceEnabled == nil {
			return nil, &runtimemanager.Error{
				Code:    "plugin.protocol_violation",
				Message: "governance.whitelist.write is missing enabled",
			}
		}
		response, err := req.Service.SetWhitelistEnabled(ctx, *req.Action.GovernanceEnabled)
		if err != nil {
			return nil, mapRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{
			"enabled": response.Enabled,
		}, nil
	case "upsert":
		entry, err := req.Service.UpsertWhitelistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{
			"entry_type": entry.EntryType,
			"target_id":  entry.TargetID,
			"reason":     entry.Reason,
			"created_at": entry.CreatedAt,
		}, nil
	case "delete":
		if err := req.Service.DeleteWhitelistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{
			"deleted": true,
		}, nil
	default:
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "governance.whitelist.write uses unsupported operation",
		}
	}
}

func ExecuteCommandPolicyRead(ctx context.Context, req Request) (map[string]any, error) {
	if err := requireCapability(ctx, req, "governance.command_policy.read"); err != nil {
		return nil, err
	}

	response, err := req.Service.ReadCommandPolicy(ctx)
	if err != nil {
		return nil, mapRuntimeError("governance.command_policy.read failed", err)
	}
	return map[string]any{
		"default_level": response.DefaultLevel,
		"cooldown":      response.Cooldown,
		"commands":      response.Commands,
	}, nil
}

func requireCapability(ctx context.Context, req Request, capability string) error {
	if req.Capabilities == nil || !req.Capabilities.CapabilityDeclared(ctx, req.PluginID, capability) {
		return &runtimemanager.Error{
			Code:    "plugin.capability_violation",
			Message: capability + " capability is not declared",
		}
	}
	if req.Service == nil {
		return &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "governance service is not available",
		}
	}
	return nil
}

func mapRuntimeError(message string, err error) error {
	switch {
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		return &runtimemanager.Error{
			Code:    "platform.resource_missing",
			Message: message,
			Err:     err,
		}
	case errors.Is(err, governance.ErrInvalidRequest):
		return &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: message,
			Err:     err,
		}
	default:
		return &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: message,
			Err:     err,
		}
	}
}
