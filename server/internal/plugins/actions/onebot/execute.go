package onebot

import (
	"context"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type Grants interface {
	CapabilityGranted(context.Context, string, string) bool
}

type Adapter interface {
	CallAPIAny(context.Context, string, map[string]any) (any, error)
	DetectedProvider() string
}

type CodedError interface {
	RuntimeActionCode() string
	RuntimeActionMessage() string
}

type Request struct {
	PluginID string
	Action   runtimeaction.Action
	Grants   Grants
	Adapter  Adapter
}

func Execute(ctx context.Context, req Request) (map[string]any, error) {
	spec, ok := Lookup(req.Action.Kind)
	if !ok {
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported local action kind",
		}
	}

	if req.Grants == nil || !req.Grants.CapabilityGranted(ctx, req.PluginID, spec.Capability) {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: spec.Capability + " capability is not granted",
		}
	}

	if req.Adapter == nil {
		return nil, &runtimemanager.Error{
			Code:    "adapter.transport_not_implemented",
			Message: "OneBot adapter 不可用",
		}
	}

	apiAction, params, err := projectAction(req.Adapter, spec, req.Action)
	if err != nil {
		return nil, err
	}

	result, callErr := req.Adapter.CallAPIAny(ctx, apiAction, params)
	if callErr != nil {
		return nil, ToRuntimeActionError(callErr)
	}
	return spec.Result(result), nil
}

func ToRuntimeActionError(err error) error {
	if err == nil {
		return nil
	}
	if adapterErr, ok := err.(CodedError); ok {
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

func projectAction(adapter Adapter, spec Spec, action runtimeaction.Action) (string, map[string]any, error) {
	if spec.Provider == "" {
		return spec.Project(action.RawData)
	}

	provider := adapter.DetectedProvider()
	if provider != spec.Provider {
		return "", nil, &runtimemanager.Error{
			Code:    "adapter.provider_extension_not_supported",
			Message: "当前 provider 不支持该扩展动作",
		}
	}

	return spec.Project(action.RawData)
}
