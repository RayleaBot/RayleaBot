package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type OneBotActionSpec struct {
	Kind          string
	Capability    string
	Provider      string
	APIName       string
	Validate      func(map[string]any) error
	Project       func(map[string]any) (string, map[string]any, error)
	Result        func(any) map[string]any
	CollectionKey string
}

var oneBotActions = buildOneBotActionRegistry()

func oneBotRegistrars() []registrar {
	registrars := make([]registrar, 0, len(oneBotActions))
	for kind, spec := range OneBotActionRegistry() {
		kind := kind
		spec := spec
		registrars = append(registrars, registrar{
			metadata: Metadata{
				Action:         kind,
				Capability:     spec.Capability,
				RequestSchema:  "plugin-protocol.onebot_action",
				ResponseSchema: "plugin-protocol.local_action_result",
				AuditFields:    []string{"plugin_id", "action", "provider"},
				ErrorCodes:     commonErrorCodes("adapter.transport_not_implemented", "adapter.provider_extension_not_supported"),
			},
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeOneBotAction(ctx, oneBotActionRequest{
						PluginID:     req.PluginID,
						Action:       req.Action,
						Capabilities: deps.Capabilities,
						Adapter:      deps.Adapter,
					})
				}
			},
		})
	}
	return registrars
}

func OneBotActionRegistry() map[string]OneBotActionSpec {
	items := make(map[string]OneBotActionSpec, len(oneBotActions))
	for kind, spec := range oneBotActions {
		items[kind] = spec
	}
	return items
}

func LookupOneBotAction(kind string) (OneBotActionSpec, bool) {
	spec, ok := oneBotActions[kind]
	return spec, ok
}

func IsOneBotLocalAction(kind string) bool {
	spec, ok := LookupOneBotAction(kind)
	return ok && spec.Provider == ""
}

func IsOneBotProviderExtensionAction(kind string) bool {
	spec, ok := LookupOneBotAction(kind)
	return ok && spec.Provider != ""
}

func buildOneBotActionRegistry() map[string]OneBotActionSpec {
	baseSpecs := onebot11.Actions()
	items := make(map[string]OneBotActionSpec, len(baseSpecs))
	for _, baseSpec := range baseSpecs {
		spec := normalizeOneBotActionSpec(oneBotActionSpecFromProtocol(baseSpec))
		items[spec.Kind] = spec
	}
	return items
}

var oneBotActionProjectors = map[string]func(map[string]any) (string, map[string]any, error){
	"message.history.get":  projectMessageHistoryGet,
	"message.forward.get":  projectMessageForwardGet,
	"message.forward.send": projectMessageForwardSend,
	"message.read.mark":    projectMessageReadMark,
	"group.ban.set":        projectGroupBanSet,
	"file.group.fs.list":   projectGroupFilesList,
	"file.group.fs.delete": projectGroupFilesDelete,
}

func oneBotActionSpecFromProtocol(baseSpec onebot11.ActionSpec) OneBotActionSpec {
	spec := OneBotActionSpec{
		Kind:          baseSpec.Kind,
		Capability:    baseSpec.Capability,
		Provider:      baseSpec.Provider,
		APIName:       baseSpec.APIName,
		CollectionKey: baseSpec.CollectionKey,
	}
	if len(baseSpec.RequiredFields) > 0 {
		spec.Validate = requireOneBotActionFields(baseSpec.RequiredFields...)
	}
	if baseSpec.NoParams {
		apiName := baseSpec.APIName
		spec.Project = func(map[string]any) (string, map[string]any, error) {
			return apiName, nil, nil
		}
	}
	if project, ok := oneBotActionProjectors[baseSpec.Kind]; ok {
		spec.Project = project
	}
	return spec
}

func normalizeOneBotActionSpec(spec OneBotActionSpec) OneBotActionSpec {
	if spec.Capability == "" {
		spec.Capability = spec.Kind
	}
	if spec.Result == nil {
		spec.Result = func(result any) map[string]any {
			return defaultResult(spec.CollectionKey, result)
		}
	}
	if spec.Project == nil {
		spec.Project = func(raw map[string]any) (string, map[string]any, error) {
			if spec.Validate != nil {
				if err := spec.Validate(raw); err != nil {
					return "", nil, err
				}
			}
			params, err := normalizeParams(raw)
			if err != nil {
				return "", nil, err
			}
			return spec.APIName, params, nil
		}
	}
	return spec
}

func requireOneBotActionFields(keys ...string) func(map[string]any) error {
	return func(raw map[string]any) error {
		for _, key := range keys {
			if _, err := requiredString(raw, key); err != nil {
				return err
			}
		}
		return nil
	}
}

type oneBotCodedError interface {
	RuntimeActionCode() string
	RuntimeActionMessage() string
}

type oneBotActionRequest struct {
	PluginID     string
	Action       pluginruntime.Action
	Capabilities CapabilityView
	Adapter      OneBotAdapter
}

func executeOneBotAction(ctx context.Context, req oneBotActionRequest) (map[string]any, error) {
	spec, ok := LookupOneBotAction(req.Action.Kind)
	if !ok {
		return nil, &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported local action kind",
		}
	}

	if req.Capabilities == nil || !req.Capabilities.CapabilityDeclared(ctx, req.PluginID, spec.Capability) {
		return nil, &pluginruntime.Error{
			Code:    "plugin.capability_violation",
			Message: spec.Capability + " capability is not declared",
		}
	}

	if req.Adapter == nil {
		return nil, &pluginruntime.Error{
			Code:    "adapter.transport_not_implemented",
			Message: "OneBot adapter 不可用",
		}
	}

	apiAction, params, err := projectOneBotAction(req.Adapter, spec, req.Action)
	if err != nil {
		return nil, err
	}

	result, callErr := req.Adapter.CallAPIAny(ctx, apiAction, params)
	if callErr != nil {
		return nil, oneBotRuntimeActionError(callErr)
	}
	return spec.Result(result), nil
}

func oneBotRuntimeActionError(err error) error {
	if err == nil {
		return nil
	}
	if adapterErr, ok := err.(oneBotCodedError); ok {
		return &pluginruntime.Error{
			Code:    adapterErr.RuntimeActionCode(),
			Message: adapterErr.RuntimeActionMessage(),
		}
	}
	return &pluginruntime.Error{
		Code:    "adapter.transport_not_implemented",
		Message: err.Error(),
	}
}

func projectOneBotAction(adapter OneBotAdapter, spec OneBotActionSpec, action pluginruntime.Action) (string, map[string]any, error) {
	if spec.Provider == "" {
		return spec.Project(action.RawData)
	}

	provider := adapter.DetectedProvider()
	if provider != spec.Provider {
		return "", nil, &pluginruntime.Error{
			Code:    "adapter.provider_extension_not_supported",
			Message: "当前 provider 不支持该扩展动作",
		}
	}

	return spec.Project(action.RawData)
}
