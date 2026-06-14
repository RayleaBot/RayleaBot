package render

import (
	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/plugins"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type Service = renderservice.Service

func NewService(options Options) (*Service, error) {
	return renderservice.NewService(options)
}

func NewChromiumRunner(options ChromiumOptions) Runner {
	return renderservice.NewChromiumRunner(options)
}

func ValidatePluginTemplateSources(sources []PluginTemplateSource) error {
	return renderplugins.ValidateSources(sources)
}

func PluginTemplateSourcesFromManifests(items []PluginTemplateSource) []PluginTemplateSource {
	return renderplugins.SourcesFromManifests(items)
}
