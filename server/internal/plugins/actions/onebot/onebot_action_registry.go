package onebot

import protocolonebot "github.com/RayleaBot/RayleaBot/server/internal/protocol/onebot"

type Spec struct {
	Kind          string
	Capability    string
	Provider      string
	APIName       string
	Validate      func(map[string]any) error
	Project       func(map[string]any) (string, map[string]any, error)
	Result        func(any) map[string]any
	CollectionKey string
}

var registry = buildRegistry()

func Registry() map[string]Spec {
	items := make(map[string]Spec, len(registry))
	for kind, spec := range registry {
		items[kind] = spec
	}
	return items
}

func Lookup(kind string) (Spec, bool) {
	spec, ok := registry[kind]
	return spec, ok
}

func IsLocalAction(kind string) bool {
	spec, ok := Lookup(kind)
	return ok && spec.Provider == ""
}

func IsProviderExtensionAction(kind string) bool {
	spec, ok := Lookup(kind)
	return ok && spec.Provider != ""
}

func buildRegistry() map[string]Spec {
	baseSpecs := protocolonebot.Actions()
	items := make(map[string]Spec, len(baseSpecs))
	for _, baseSpec := range baseSpecs {
		spec := normalizeSpec(specFromProtocol(baseSpec))
		items[spec.Kind] = spec
	}
	return items
}

var projectors = map[string]func(map[string]any) (string, map[string]any, error){
	"message.history.get":  projectMessageHistoryGet,
	"message.forward.get":  projectMessageForwardGet,
	"message.forward.send": projectMessageForwardSend,
	"message.read.mark":    projectMessageReadMark,
	"group.ban.set":        projectGroupBanSet,
	"file.group.fs.list":   projectGroupFilesList,
	"file.group.fs.delete": projectGroupFilesDelete,
}

func specFromProtocol(baseSpec protocolonebot.ActionSpec) Spec {
	spec := Spec{
		Kind:          baseSpec.Kind,
		Capability:    baseSpec.Capability,
		Provider:      baseSpec.Provider,
		APIName:       baseSpec.APIName,
		CollectionKey: baseSpec.CollectionKey,
	}
	if len(baseSpec.RequiredFields) > 0 {
		spec.Validate = requireFields(baseSpec.RequiredFields...)
	}
	if baseSpec.NoParams {
		apiName := baseSpec.APIName
		spec.Project = func(map[string]any) (string, map[string]any, error) {
			return apiName, nil, nil
		}
	}
	if project, ok := projectors[baseSpec.Kind]; ok {
		spec.Project = project
	}
	return spec
}

func normalizeSpec(spec Spec) Spec {
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

func requireFields(keys ...string) func(map[string]any) error {
	return func(raw map[string]any) error {
		for _, key := range keys {
			if _, err := requiredString(raw, key); err != nil {
				return err
			}
		}
		return nil
	}
}
