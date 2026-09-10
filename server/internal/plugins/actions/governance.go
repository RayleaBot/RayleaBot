package actions

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func governanceRegistrars() []registrar {
	return []registrar{
		governanceRegistrar("governance.blacklist.read", blacklistRead),
		governanceRegistrar("governance.blacklist.write", blacklistWrite),
		governanceRegistrar("governance.whitelist.read", whitelistRead),
		governanceRegistrar("governance.whitelist.write", whitelistWrite),
		governanceRegistrar("governance.command_policy.read", commandPolicyRead),
	}
}

func governanceRegistrar(action string, execute func(context.Context, Deps, ActionRequest) (map[string]any, error)) registrar {
	return registrar{
		kind: action,
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return execute(ctx, deps, req)
			}
		},
	}
}

type GovernanceService interface {
	ReadBlacklist(context.Context) (governance.BlacklistSnapshot, error)
	UpsertBlacklistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteBlacklistEntry(context.Context, string, string) error
	ReadWhitelist(context.Context) (governance.WhitelistSnapshot, error)
	SetWhitelistEnabled(context.Context, bool) (governance.WhitelistStateResponse, error)
	UpsertWhitelistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteWhitelistEntry(context.Context, string, string) error
	ReadCommandPolicy(context.Context) (governance.CommandPolicyResponse, error)
}

func requireGovernancePermission(ctx context.Context, deps Deps, req ActionRequest, permission string) (GovernanceService, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, permission) {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: permission + " permission is not declared"}
	}
	service := deps.Governance
	if service == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "governance service is not available"}
	}
	return service, nil
}

func mapGovernanceRuntimeError(message string, err error) error {
	switch {
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		return &pluginruntime.Error{Code: "platform.resource_missing", Message: message, Err: err}
	case errors.Is(err, governance.ErrInvalidRequest):
		return &pluginruntime.Error{Code: "plugin.protocol_violation", Message: message, Err: err}
	default:
		return &pluginruntime.Error{Code: "plugin.internal_error", Message: message, Err: err}
	}
}

func blacklistRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	service, err := requireGovernancePermission(ctx, deps, req, "governance.blacklist.read")
	if err != nil {
		return nil, err
	}
	snapshot, err := service.ReadBlacklist(ctx)
	if err != nil {
		return nil, mapGovernanceRuntimeError("governance.blacklist.read failed", err)
	}
	return map[string]any{"user_entries": snapshot.UserEntries, "group_entries": snapshot.GroupEntries}, nil
}

func blacklistWrite(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	service, err := requireGovernancePermission(ctx, deps, req, "governance.blacklist.write")
	if err != nil {
		return nil, err
	}
	switch req.Action.GovernanceOperation {
	case "upsert":
		entry, err := service.UpsertBlacklistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{"entry_type": entry.EntryType, "target_id": entry.TargetID, "reason": entry.Reason, "created_at": entry.CreatedAt}, nil
	case "delete":
		if err := service.DeleteBlacklistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapGovernanceRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{"deleted": true}, nil
	default:
		return nil, &pluginruntime.Error{Code: "plugin.protocol_violation", Message: "governance.blacklist.write uses unsupported operation"}
	}
}

func whitelistRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	service, err := requireGovernancePermission(ctx, deps, req, "governance.whitelist.read")
	if err != nil {
		return nil, err
	}
	snapshot, err := service.ReadWhitelist(ctx)
	if err != nil {
		return nil, mapGovernanceRuntimeError("governance.whitelist.read failed", err)
	}
	return map[string]any{"enabled": snapshot.Enabled, "user_entries": snapshot.UserEntries, "group_entries": snapshot.GroupEntries}, nil
}

func whitelistWrite(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	service, err := requireGovernancePermission(ctx, deps, req, "governance.whitelist.write")
	if err != nil {
		return nil, err
	}
	switch req.Action.GovernanceOperation {
	case "set_enabled":
		if req.Action.GovernanceEnabled == nil {
			return nil, &pluginruntime.Error{Code: "plugin.protocol_violation", Message: "governance.whitelist.write is missing enabled"}
		}
		response, err := service.SetWhitelistEnabled(ctx, *req.Action.GovernanceEnabled)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"enabled": response.Enabled}, nil
	case "upsert":
		entry, err := service.UpsertWhitelistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"entry_type": entry.EntryType, "target_id": entry.TargetID, "reason": entry.Reason, "created_at": entry.CreatedAt}, nil
	case "delete":
		if err := service.DeleteWhitelistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"deleted": true}, nil
	default:
		return nil, &pluginruntime.Error{Code: "plugin.protocol_violation", Message: "governance.whitelist.write uses unsupported operation"}
	}
}

func commandPolicyRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	service, err := requireGovernancePermission(ctx, deps, req, "governance.command_policy.read")
	if err != nil {
		return nil, err
	}
	response, err := service.ReadCommandPolicy(ctx)
	if err != nil {
		return nil, mapGovernanceRuntimeError("governance.command_policy.read failed", err)
	}
	return map[string]any{"default_level": response.DefaultLevel, "cooldown": response.Cooldown, "commands": response.Commands}, nil
}
