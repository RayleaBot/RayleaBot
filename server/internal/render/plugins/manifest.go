package plugins

import (
	"fmt"
	"strings"

	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func ValidateSources(sources []Source) error {
	seenTemplates := map[string]struct{}{}
	for _, source := range sources {
		pluginID := strings.TrimSpace(source.PluginID)
		dir := strings.TrimSpace(source.Dir)
		if pluginID == "" || dir == "" {
			return fmt.Errorf("plugin render template declaration is incomplete")
		}
		_, localID, err := loadValidSeed(pluginID, dir)
		if err != nil {
			return err
		}
		templateID := FormalID(pluginID, localID)
		if _, ok := seenTemplates[templateID]; ok {
			return fmt.Errorf("duplicate plugin render template id %s", templateID)
		}
		seenTemplates[templateID] = struct{}{}
	}
	return nil
}

func SourcesFromManifests(items []Source) []Source {
	sources := make([]Source, 0, len(items))
	for _, item := range items {
		pluginID := strings.TrimSpace(item.PluginID)
		dir := strings.TrimSpace(item.Dir)
		if pluginID == "" || dir == "" {
			continue
		}
		_, localID, err := loadValidSeed(pluginID, dir)
		if err != nil {
			continue
		}
		item.PluginID = pluginID
		item.LocalID = localID
		item.Dir = dir
		item.ResourceRoot = strings.TrimSpace(item.ResourceRoot)
		sources = append(sources, item)
	}
	return sources
}

func loadValidSeed(pluginID, dir string) (rendertemplates.Seed, string, error) {
	seed, err := rendertemplates.LoadSeed(dir)
	if err != nil {
		return rendertemplates.Seed{}, "", fmt.Errorf("load plugin render template %s: %w", pluginID, err)
	}
	localID := strings.TrimSpace(seed.Compiled.Bundle.Manifest.ID)
	if !IsValidLocalID(localID) {
		return rendertemplates.Seed{}, "", fmt.Errorf("plugin render template %s has invalid local id %q", pluginID, localID)
	}
	return seed, localID, nil
}
