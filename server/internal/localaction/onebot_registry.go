package localaction

import protocolonebot "github.com/RayleaBot/RayleaBot/server/internal/protocol/onebot"

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

var oneBotActionRegistry = buildOneBotActionRegistry()

func buildOneBotActionRegistry() map[string]OneBotActionSpec {
	baseSpecs := protocolonebot.Actions()
	registry := make(map[string]OneBotActionSpec, len(baseSpecs))
	for _, baseSpec := range baseSpecs {
		spec := normalizeOneBotActionSpec(oneBotActionSpecFromProtocol(baseSpec))
		registry[spec.Kind] = spec
	}
	return registry
}

var oneBotActionProjectors = map[string]func(map[string]any) (string, map[string]any, error){
	"message.history.get":  projectOneBotMessageHistoryGet,
	"message.forward.get":  projectOneBotMessageForwardGet,
	"message.forward.send": projectOneBotMessageForwardSend,
	"message.read.mark":    projectOneBotMessageReadMark,
	"group.ban.set":        projectOneBotGroupBanSet,
	"file.group.fs.list":   projectOneBotGroupFilesList,
	"file.group.fs.delete": projectOneBotGroupFilesDelete,
}

func oneBotActionSpecFromProtocol(baseSpec protocolonebot.ActionSpec) OneBotActionSpec {
	spec := OneBotActionSpec{
		Kind:          baseSpec.Kind,
		Capability:    baseSpec.Capability,
		Provider:      baseSpec.Provider,
		APIName:       baseSpec.APIName,
		CollectionKey: baseSpec.CollectionKey,
	}
	if len(baseSpec.RequiredFields) > 0 {
		spec.Validate = requireOneBotFields(baseSpec.RequiredFields...)
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
			return defaultOneBotActionResult(spec.CollectionKey, result)
		}
	}
	if spec.Project == nil {
		spec.Project = func(raw map[string]any) (string, map[string]any, error) {
			if spec.Validate != nil {
				if err := spec.Validate(raw); err != nil {
					return "", nil, err
				}
			}
			params, err := normalizeActionParams(raw)
			if err != nil {
				return "", nil, err
			}
			return spec.APIName, params, nil
		}
	}
	return spec
}

func requireOneBotFields(keys ...string) func(map[string]any) error {
	return func(raw map[string]any) error {
		for _, key := range keys {
			if _, err := requiredActionString(raw, key); err != nil {
				return err
			}
		}
		return nil
	}
}

func lookupOneBotActionSpec(kind string) (OneBotActionSpec, bool) {
	spec, ok := oneBotActionRegistry[kind]
	return spec, ok
}
