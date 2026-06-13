package localaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func runtimeIsOneBotLocalAction(kind string) bool {
	spec, ok := lookupOneBotActionSpec(kind)
	return ok && spec.Provider == ""
}

func runtimeIsProviderExtensionAction(kind string) bool {
	spec, ok := lookupOneBotActionSpec(kind)
	return ok && spec.Provider != ""
}

func (s *Service) executeOneBotLocalAction(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	spec, ok := lookupOneBotActionSpec(action.Kind)
	if !ok {
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported local action kind",
		}
	}

	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, spec.Capability) {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: spec.Capability + " capability is not granted",
		}
	}

	if s.adapter == nil {
		return nil, &runtime.Error{
			Code:    "adapter.transport_not_implemented",
			Message: "OneBot adapter 不可用",
		}
	}

	apiAction, params, err := s.projectOneBotAction(ctx, spec, action)
	if err != nil {
		return nil, err
	}

	result, callErr := s.adapter.CallAPIAny(ctx, apiAction, params)
	if callErr != nil {
		return nil, toRuntimeActionError(callErr)
	}
	return spec.Result(result), nil
}

func toRuntimeActionError(err error) error {
	if err == nil {
		return nil
	}
	if adapterErr, ok := err.(*adapter.Error); ok {
		return &runtime.Error{
			Code:    adapterErr.Code,
			Message: adapterErr.Message,
		}
	}
	return &runtime.Error{
		Code:    "adapter.transport_not_implemented",
		Message: err.Error(),
	}
}

func (s *Service) projectOneBotAction(ctx context.Context, spec OneBotActionSpec, action runtime.Action) (string, map[string]any, error) {
	if spec.Provider == "" {
		return spec.Project(action.RawData)
	}

	provider := s.adapter.Snapshot().DetectedProvider()
	if provider != spec.Provider {
		return "", nil, &runtime.Error{
			Code:    "adapter.provider_extension_not_supported",
			Message: "当前 provider 不支持该扩展动作",
		}
	}

	return spec.Project(action.RawData)
}
