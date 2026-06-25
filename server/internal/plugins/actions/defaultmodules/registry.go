package defaultmodules

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

type Metadata struct {
	Action             string
	Capability         string
	RequestSchema      string
	ResponseSchema     string
	RequiredPermission string
	ReadsSecret        bool
	WritesSecret       bool
	AccessesNetwork    bool
	WritesFile         bool
	AuditFields        []string
	ErrorCodes         []string
}

type registrar struct {
	metadata Metadata
	factory  func(actions.Deps) actions.ActionHandler
}

func (r registrar) RegisterActions(registry *actions.Registry, deps actions.Deps) {
	if registry == nil || r.factory == nil || r.metadata.Action == "" {
		return
	}
	registry.Register(r.metadata.Action, r.factory(deps))
}

var registrars []registrar

func register(metadata Metadata, factory func(actions.Deps) actions.ActionHandler) {
	registrars = append(registrars, registrar{metadata: metadata, factory: factory})
}

func Registrars() []actions.Registrar {
	items := make([]actions.Registrar, 0, len(registrars))
	for _, item := range registrars {
		items = append(items, item)
	}
	return items
}

func NewRegistry(deps actions.Deps) *actions.Registry {
	return actions.NewRegistryWithRegistrars(deps, Registrars()...)
}

func MetadataList() []Metadata {
	items := make([]Metadata, 0, len(registrars))
	for _, item := range registrars {
		metadata := item.metadata
		metadata.AuditFields = append([]string(nil), metadata.AuditFields...)
		metadata.ErrorCodes = append([]string(nil), metadata.ErrorCodes...)
		items = append(items, metadata)
	}
	return items
}

func commonErrorCodes(extra ...string) []string {
	codes := []string{
		"plugin.capability_violation",
		"plugin.internal_error",
		"plugin.protocol_violation",
	}
	return append(codes, extra...)
}
