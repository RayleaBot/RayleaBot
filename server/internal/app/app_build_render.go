package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

func buildRenderService(state appBuildState, platform appPlatform, renderRunner render.Runner) (*render.Service, error) {
	renderBrowserPath := prepareRenderBrowserPath(context.Background(), state.core.Logger, state.discoverySpec.repoRoot, state.core.Config.Render.BrowserPath)
	renderService, err := render.NewService(render.Options{
		RepoRoot:           state.discoverySpec.repoRoot,
		OutputRoot:         filepath.Join(filepath.Dir(platform.Storage.Path), "render"),
		Store:              platform.Storage,
		Runner:             renderRunner,
		WorkerCount:        state.core.Config.Render.WorkerCount,
		BrowserArgs:        state.core.Config.Render.BrowserArgs,
		BrowserPath:        renderBrowserPath,
		QueueMaxLength:     state.core.Config.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(state.core.Config.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(state.core.Config.Render.TimeoutSeconds) * time.Second,
		MaxRenderDataBytes: int(httpapi.MaxManagementJSONBodyBytes),
		FooterTemplate:     state.core.Config.Render.FooterTemplate,
		DefaultOutput:      state.core.Config.Render.DefaultOutput,
		DeviceScalePercent: state.core.Config.Render.DeviceScalePercent,
		Logger:             state.core.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create render service: %w", err)
	}
	return renderService, nil
}

func syncCatalogRenderTemplates(ctx context.Context, renderer *render.Service, catalog *plugins.Catalog) error {
	if renderer == nil || catalog == nil {
		return nil
	}
	return renderer.SyncPluginTemplates(ctx, pluginRenderTemplateSources(catalog.List()))
}

func pluginRenderTemplateSources(snapshots []plugins.Snapshot) []render.PluginTemplateSource {
	var sources []render.PluginTemplateSource
	seen := map[string]struct{}{}
	for _, snapshot := range snapshots {
		if !snapshot.Valid || snapshot.RegistrationState != "installed" {
			continue
		}
		for _, declared := range snapshot.RenderTemplates {
			dir, ok := pluginPackageRelativeDir(snapshot.PackageRootPath, declared.Path)
			if !ok {
				continue
			}
			key := snapshot.PluginID + "\x00" + dir
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sources = append(sources, render.PluginTemplateSource{
				PluginID:     snapshot.PluginID,
				Dir:          dir,
				ResourceRoot: snapshot.PackageRootPath,
			})
		}
	}
	return render.PluginTemplateSourcesFromManifests(sources)
}

func validatePluginRenderTemplates(snapshot plugins.Snapshot) error {
	var sources []render.PluginTemplateSource
	for _, declared := range snapshot.RenderTemplates {
		dir, ok := pluginPackageRelativeDir(snapshot.PackageRootPath, declared.Path)
		if !ok {
			return fmt.Errorf("plugin render template path %q is invalid", declared.Path)
		}
		sources = append(sources, render.PluginTemplateSource{
			PluginID:     snapshot.PluginID,
			Dir:          dir,
			ResourceRoot: snapshot.PackageRootPath,
		})
	}
	return render.ValidatePluginTemplateSources(sources)
}

func pluginPackageRelativeDir(packageRoot, relativePath string) (string, bool) {
	packageRoot = strings.TrimSpace(packageRoot)
	relativePath = strings.TrimSpace(relativePath)
	if packageRoot == "" || relativePath == "" || filepath.IsAbs(relativePath) {
		return "", false
	}
	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", false
	}
	absoluteRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(absoluteRoot, cleanRelative)
	relativeToRoot, err := filepath.Rel(absoluteRoot, candidate)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", false
	}
	return candidate, true
}
