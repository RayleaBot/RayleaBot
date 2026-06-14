package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/plugins"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func SyncCatalogRenderTemplates(ctx context.Context, renderer *renderservice.Service, catalog *plugincatalog.Catalog) error {
	if renderer == nil || catalog == nil {
		return nil
	}
	return renderer.SyncPluginTemplates(ctx, pluginRenderTemplateSources(catalog.List()))
}

func ValidatePluginRenderTemplates(snapshot plugins.Snapshot) error {
	var sources []renderplugins.Source
	for _, declared := range snapshot.RenderTemplates {
		dir, ok := pluginPackageRelativeDir(snapshot.PackageRootPath, declared.Path)
		if !ok {
			return fmt.Errorf("plugin render template path %q is invalid", declared.Path)
		}
		sources = append(sources, renderplugins.Source{
			PluginID:     snapshot.PluginID,
			Dir:          dir,
			ResourceRoot: snapshot.PackageRootPath,
		})
	}
	return renderplugins.ValidateSources(sources)
}

func pluginRenderTemplateSources(snapshots []plugins.Snapshot) []renderplugins.Source {
	var sources []renderplugins.Source
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
			sources = append(sources, renderplugins.Source{
				PluginID:     snapshot.PluginID,
				Dir:          dir,
				ResourceRoot: snapshot.PackageRootPath,
			})
		}
	}
	return renderplugins.SourcesFromManifests(sources)
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
