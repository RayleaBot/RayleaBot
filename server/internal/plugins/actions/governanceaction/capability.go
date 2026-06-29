package governanceaction

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type governanceService interface {
	ReadBlacklist(context.Context) (governance.BlacklistSnapshot, error)
	UpsertBlacklistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteBlacklistEntry(context.Context, string, string) error
	ReadWhitelist(context.Context) (governance.WhitelistSnapshot, error)
	SetWhitelistEnabled(context.Context, bool) (governance.WhitelistStateResponse, error)
	UpsertWhitelistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteWhitelistEntry(context.Context, string, string) error
	ReadCommandPolicy(context.Context) (governance.CommandPolicyResponse, error)
}

func requireCapability(ctx context.Context, deps actions.Deps, req actions.ActionRequest, capability string) (governanceService, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, capability) {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: capability + " capability is not declared"}
	}
	service, ok := deps.Governance.(governanceService)
	if !ok || service == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "governance service is not available"}
	}
	return service, nil
}

func mapRuntimeError(message string, err error) error {
	switch {
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		return &runtimemanager.Error{Code: "platform.resource_missing", Message: message, Err: err}
	case errors.Is(err, governance.ErrInvalidRequest):
		return &runtimemanager.Error{Code: "plugin.protocol_violation", Message: message, Err: err}
	default:
		return &runtimemanager.Error{Code: "plugin.internal_error", Message: message, Err: err}
	}
}
