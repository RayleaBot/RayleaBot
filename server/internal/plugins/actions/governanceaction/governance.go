package governanceaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func BlacklistRead(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	service, err := requireCapability(ctx, deps, req, "governance.blacklist.read")
	if err != nil {
		return nil, err
	}
	snapshot, err := service.ReadBlacklist(ctx)
	if err != nil {
		return nil, mapRuntimeError("governance.blacklist.read failed", err)
	}
	return map[string]any{"user_entries": snapshot.UserEntries, "group_entries": snapshot.GroupEntries}, nil
}

func BlacklistWrite(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	service, err := requireCapability(ctx, deps, req, "governance.blacklist.write")
	if err != nil {
		return nil, err
	}
	switch req.Action.GovernanceOperation {
	case "upsert":
		entry, err := service.UpsertBlacklistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{"entry_type": entry.EntryType, "target_id": entry.TargetID, "reason": entry.Reason, "created_at": entry.CreatedAt}, nil
	case "delete":
		if err := service.DeleteBlacklistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapRuntimeError("governance.blacklist.write failed", err)
		}
		return map[string]any{"deleted": true}, nil
	default:
		return nil, &runtimemanager.Error{Code: "plugin.protocol_violation", Message: "governance.blacklist.write uses unsupported operation"}
	}
}

func WhitelistRead(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	service, err := requireCapability(ctx, deps, req, "governance.whitelist.read")
	if err != nil {
		return nil, err
	}
	snapshot, err := service.ReadWhitelist(ctx)
	if err != nil {
		return nil, mapRuntimeError("governance.whitelist.read failed", err)
	}
	return map[string]any{"enabled": snapshot.Enabled, "user_entries": snapshot.UserEntries, "group_entries": snapshot.GroupEntries}, nil
}

func WhitelistWrite(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	service, err := requireCapability(ctx, deps, req, "governance.whitelist.write")
	if err != nil {
		return nil, err
	}
	switch req.Action.GovernanceOperation {
	case "set_enabled":
		if req.Action.GovernanceEnabled == nil {
			return nil, &runtimemanager.Error{Code: "plugin.protocol_violation", Message: "governance.whitelist.write is missing enabled"}
		}
		response, err := service.SetWhitelistEnabled(ctx, *req.Action.GovernanceEnabled)
		if err != nil {
			return nil, mapRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"enabled": response.Enabled}, nil
	case "upsert":
		entry, err := service.UpsertWhitelistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID, req.Action.GovernanceReason)
		if err != nil {
			return nil, mapRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"entry_type": entry.EntryType, "target_id": entry.TargetID, "reason": entry.Reason, "created_at": entry.CreatedAt}, nil
	case "delete":
		if err := service.DeleteWhitelistEntry(ctx, req.Action.GovernanceEntryType, req.Action.GovernanceTargetID); err != nil {
			return nil, mapRuntimeError("governance.whitelist.write failed", err)
		}
		return map[string]any{"deleted": true}, nil
	default:
		return nil, &runtimemanager.Error{Code: "plugin.protocol_violation", Message: "governance.whitelist.write uses unsupported operation"}
	}
}

func CommandPolicyRead(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	service, err := requireCapability(ctx, deps, req, "governance.command_policy.read")
	if err != nil {
		return nil, err
	}
	response, err := service.ReadCommandPolicy(ctx)
	if err != nil {
		return nil, mapRuntimeError("governance.command_policy.read failed", err)
	}
	return map[string]any{"default_level": response.DefaultLevel, "cooldown": response.Cooldown, "commands": response.Commands}, nil
}
