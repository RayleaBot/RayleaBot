package actions

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type OneBotActionSpec struct {
	Kind          string
	Permission    string
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
	for kind := range OneBotActionRegistry() {
		registrars = append(registrars, registrar{
			kind: kind,
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeOneBotAction(ctx, oneBotActionRequest{
						PluginID:       req.PluginID,
						Action:         req.Action,
						Permissions:    deps.Permissions,
						ParentEvent:    req.ParentEvent,
						ResolveAdapter: deps.ResolveOneBotAdapter,
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
	"group.member.get":     projectGroupMemberGet,
	"group.ban.set":        projectGroupBanSet,
	"file.group.fs.list":   projectGroupFilesList,
	"file.group.fs.delete": projectGroupFilesDelete,
}

func oneBotActionSpecFromProtocol(baseSpec onebot11.ActionSpec) OneBotActionSpec {
	spec := OneBotActionSpec{
		Kind:          baseSpec.Kind,
		Permission:    baseSpec.Permission,
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
	if spec.Permission == "" {
		spec.Permission = spec.Kind
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
	PluginID       string
	Action         plugins.Action
	Permissions    PermissionView
	ParentEvent    chatevent.Event
	ResolveAdapter func(string, string) (OneBotAdapter, error)
}

func executeOneBotAction(ctx context.Context, req oneBotActionRequest) (map[string]any, error) {
	spec, ok := LookupOneBotAction(req.Action.Kind)
	if !ok {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginProtocolViolation,
			Message: "received unsupported local action kind",
		}
	}

	if req.Permissions == nil || !req.Permissions.PermissionDeclared(ctx, req.PluginID, spec.Permission) {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginPermissionDenied,
			Message: spec.Permission + " permission is not declared",
		}
	}

	sourceAdapter, err := oneBotActionSource(req.Action, req.ParentEvent)
	if err != nil {
		return nil, err
	}
	if req.ResolveAdapter == nil {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginProtocolViolation,
			Message: "OneBot adapter 不可用",
		}
	}
	adapter, err := req.ResolveAdapter(sourceAdapter, "onebot11")
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: err.Error()}
	}
	if adapter == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "OneBot adapter 不可用"}
	}

	apiAction, params, err := projectOneBotAction(adapter, spec, req.Action)
	if err != nil {
		return nil, err
	}

	result, callErr := adapter.CallAPIAny(ctx, apiAction, params)
	if callErr != nil {
		return nil, oneBotRuntimeActionError(callErr)
	}
	return spec.Result(result), nil
}

func oneBotActionSource(action plugins.Action, parent chatevent.Event) (string, error) {
	invalid := func(message string) (string, error) {
		return "", &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: message}
	}
	if action.SourceProtocol != "" && action.SourceProtocol != "onebot11" {
		return invalid("OneBot action source_protocol must be onebot11")
	}
	sourceAdapter := strings.TrimSpace(action.SourceAdapter)
	switch parent.SourceProtocol {
	case "onebot11":
		parentAdapter := strings.TrimSpace(parent.SourceAdapter)
		if parentAdapter == "" {
			return invalid("OneBot parent event is missing source_adapter")
		}
		if sourceAdapter != "" && sourceAdapter != parentAdapter {
			return invalid("OneBot action source_adapter conflicts with its parent event")
		}
		sourceAdapter = parentAdapter
	case "", "platform", "scheduler", "webhook", "management":
		// Host events have no chat adapter ownership.
	default:
		return invalid("OneBot action cannot use a parent event from another chat protocol")
	}
	return sourceAdapter, nil
}

func oneBotRuntimeActionError(err error) error {
	if err == nil {
		return nil
	}
	var adapterErr oneBotCodedError
	if errors.As(err, &adapterErr) {
		return &plugins.Error{
			Code:    adapterErr.RuntimeActionCode(),
			Message: adapterErr.RuntimeActionMessage(),
		}
	}
	return &plugins.Error{
		Code:    errorcodes.AdapterTransportNotImplemented,
		Message: err.Error(),
	}
}

func projectOneBotAction(adapter OneBotAdapter, spec OneBotActionSpec, action plugins.Action) (string, map[string]any, error) {
	if spec.Provider == "" {
		return spec.Project(action.RawData)
	}

	provider := adapter.DetectedProvider()
	if provider != spec.Provider {
		return "", nil, &plugins.Error{
			Code:    errorcodes.AdapterProviderExtensionNotSupported,
			Message: "当前 provider 不支持该扩展动作",
		}
	}

	return spec.Project(action.RawData)
}
