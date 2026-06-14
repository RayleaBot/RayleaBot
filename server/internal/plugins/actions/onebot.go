package actions

import (
	"context"

	localonebot "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/onebot"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type runtimeActionCodedError interface {
	RuntimeActionCode() string
	RuntimeActionMessage() string
}

func runtimeIsOneBotLocalAction(kind string) bool {
	return localonebot.IsLocalAction(kind)
}

func runtimeIsProviderExtensionAction(kind string) bool {
	return localonebot.IsProviderExtensionAction(kind)
}

func (s *Service) executeOneBotLocalAction(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	spec, ok := localonebot.Lookup(action.Kind)
	if !ok {
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported local action kind",
		}
	}

	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, spec.Capability) {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: spec.Capability + " capability is not granted",
		}
	}

	if s.adapter == nil {
		return nil, &runtimemanager.Error{
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
	if adapterErr, ok := err.(runtimeActionCodedError); ok {
		return &runtimemanager.Error{
			Code:    adapterErr.RuntimeActionCode(),
			Message: adapterErr.RuntimeActionMessage(),
		}
	}
	return &runtimemanager.Error{
		Code:    "adapter.transport_not_implemented",
		Message: err.Error(),
	}
}

func (s *Service) projectOneBotAction(ctx context.Context, spec localonebot.Spec, action runtimeaction.Action) (string, map[string]any, error) {
	if spec.Provider == "" {
		return spec.Project(action.RawData)
	}

	provider := s.adapter.DetectedProvider()
	if provider != spec.Provider {
		return "", nil, &runtimemanager.Error{
			Code:    "adapter.provider_extension_not_supported",
			Message: "当前 provider 不支持该扩展动作",
		}
	}

	return spec.Project(action.RawData)
}
