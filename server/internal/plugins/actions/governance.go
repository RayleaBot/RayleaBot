package actions

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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
	UpsertBlacklistEntry(context.Context, chatevent.IdentityScope, string, string, string) (governance.EntryResponse, error)
	DeleteBlacklistEntry(context.Context, chatevent.IdentityScope, string, string) error
	ReadWhitelist(context.Context) (governance.WhitelistSnapshot, error)
	SetWhitelistEnabled(context.Context, bool) (governance.WhitelistStateResponse, error)
	UpsertWhitelistEntry(context.Context, chatevent.IdentityScope, string, string, string) (governance.EntryResponse, error)
	DeleteWhitelistEntry(context.Context, chatevent.IdentityScope, string, string) error
	ReadCommandPolicy(context.Context) (governance.CommandPolicyResponse, error)
}

func requireGovernancePermission(ctx context.Context, deps Deps, req ActionRequest, permission string) (GovernanceService, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, permission) {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: permission + " permission is not declared"}
	}
	service := deps.Governance
	if service == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "governance service is not available"}
	}
	return service, nil
}

func mapGovernanceRuntimeError(message string, err error) error {
	switch {
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		return &plugins.Error{Code: errorcodes.PlatformResourceMissing, Message: message, Err: err}
	case errors.Is(err, governance.ErrInvalidRequest):
		return &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: message, Err: err}
	default:
		return &plugins.Error{Code: errorcodes.PluginInternalError, Message: message, Err: err}
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
		entry, err := service.UpsertBlacklistEntry(ctx, req.Action.GovernanceScope, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{"entry_type": entry.EntryType, "target_id": entry.TargetID, "reason": entry.Reason, "created_at": entry.CreatedAt}, nil
	case "delete":
		if err := service.DeleteBlacklistEntry(ctx, req.Action.GovernanceScope, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapGovernanceRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{"deleted": true}, nil
	default:
		return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "governance.blacklist.write uses unsupported operation"}
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
			return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "governance.whitelist.write is missing enabled"}
		}
		response, err := service.SetWhitelistEnabled(ctx, *req.Action.GovernanceEnabled)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"enabled": response.Enabled}, nil
	case "upsert":
		entry, err := service.UpsertWhitelistEntry(ctx, req.Action.GovernanceScope, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"entry_type": entry.EntryType, "target_id": entry.TargetID, "reason": entry.Reason, "created_at": entry.CreatedAt}, nil
	case "delete":
		if err := service.DeleteWhitelistEntry(ctx, req.Action.GovernanceScope, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapGovernanceRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"deleted": true}, nil
	default:
		return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "governance.whitelist.write uses unsupported operation"}
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
