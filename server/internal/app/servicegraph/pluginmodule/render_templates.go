package pluginmodule

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func pluginRenderTemplateSync(deps ServiceDeps) func(context.Context) error {
	return func(ctx context.Context) error {
		if deps.Renderer == nil || deps.Plugins.Plugins == nil {
			return nil
		}
		return deps.Renderer.SyncPluginTemplateDeclarations(ctx, pluginRenderTemplateDeclarations(deps.Plugins.Plugins.List()))
	}
}

func pluginRenderTemplateDeclarations(snapshots []plugins.Snapshot) []renderservice.PluginTemplateDeclaration {
	var declarations []renderservice.PluginTemplateDeclaration
	for _, snapshot := range snapshots {
		for _, declared := range snapshot.RenderTemplates {
			declarations = append(declarations, renderservice.PluginTemplateDeclaration{
				PluginID:          snapshot.PluginID,
				Path:              declared.Path,
				PackageRootPath:   snapshot.PackageRootPath,
				Valid:             snapshot.Valid,
				RegistrationState: snapshot.RegistrationState,
			})
		}
	}
	return declarations
}
