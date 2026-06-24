package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/pluginsync"
)

type PluginTemplateDeclaration struct {
	PluginID          string
	Path              string
	PackageRootPath   string
	Valid             bool
	RegistrationState string
}

func (s *Service) SyncPluginTemplateDeclarations(ctx context.Context, declarations []PluginTemplateDeclaration) error {
	return s.SyncPluginTemplates(ctx, pluginTemplateSourcesFromDeclarations(declarations))
}

func ValidatePluginTemplateDeclarations(declarations []PluginTemplateDeclaration) error {
	sources := make([]renderplugins.Source, 0, len(declarations))
	for _, declaration := range declarations {
		source, ok := pluginTemplateSource(declaration)
		if !ok {
			return fmt.Errorf("plugin render template path %q is invalid", declaration.Path)
		}
		sources = append(sources, source)
	}
	return renderplugins.ValidateSources(sources)
}

func pluginTemplateSourcesFromDeclarations(declarations []PluginTemplateDeclaration) []renderplugins.Source {
	sources := make([]renderplugins.Source, 0, len(declarations))
	seen := map[string]struct{}{}
	for _, declaration := range declarations {
		if !declaration.Valid || declaration.RegistrationState != "installed" {
			continue
		}
		source, ok := pluginTemplateSource(declaration)
		if !ok {
			continue
		}
		key := source.PluginID + "\x00" + source.Dir
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, source)
	}
	return renderplugins.SourcesFromManifests(sources)
}

func pluginTemplateSource(declaration PluginTemplateDeclaration) (renderplugins.Source, bool) {
	pluginID := strings.TrimSpace(declaration.PluginID)
	packageRoot := strings.TrimSpace(declaration.PackageRootPath)
	relativePath := strings.TrimSpace(declaration.Path)
	if pluginID == "" || packageRoot == "" || relativePath == "" || filepath.IsAbs(relativePath) {
		return renderplugins.Source{}, false
	}
	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return renderplugins.Source{}, false
	}
	absoluteRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return renderplugins.Source{}, false
	}
	candidate := filepath.Join(absoluteRoot, cleanRelative)
	relativeToRoot, err := filepath.Rel(absoluteRoot, candidate)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return renderplugins.Source{}, false
	}
	return renderplugins.Source{
		PluginID:     pluginID,
		Dir:          candidate,
		ResourceRoot: packageRoot,
	}, true
}
