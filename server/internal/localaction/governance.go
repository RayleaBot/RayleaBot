package localaction

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

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
