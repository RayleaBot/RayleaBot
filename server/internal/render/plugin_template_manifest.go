package render

import (
	"fmt"
	"strings"
)

func ValidatePluginTemplateSources(sources []PluginTemplateSource) error {
	seenTemplates := map[string]struct{}{}
	for _, source := range sources {
		pluginID := strings.TrimSpace(source.PluginID)
		dir := strings.TrimSpace(source.Dir)
		if pluginID == "" || dir == "" {
			return fmt.Errorf("plugin render template declaration is incomplete")
		}
		seed, err := loadTemplateSeed(dir)
		if err != nil {
			return fmt.Errorf("load plugin render template %s: %w", pluginID, err)
		}
		localID := strings.TrimSpace(seed.compiled.bundle.manifest.ID)
		if !pluginTemplateLocalIDPattern.MatchString(localID) {
			return fmt.Errorf("plugin render template %s has invalid local id %q", pluginID, localID)
		}
		templateID := formalPluginTemplateID(pluginID, localID)
		if _, ok := seenTemplates[templateID]; ok {
			return fmt.Errorf("duplicate plugin render template id %s", templateID)
		}
		seenTemplates[templateID] = struct{}{}
	}
	return nil
}

func PluginTemplateSourcesFromManifests(items []PluginTemplateSource) []PluginTemplateSource {
	sources := make([]PluginTemplateSource, 0, len(items))
	for _, item := range items {
		pluginID := strings.TrimSpace(item.PluginID)
		dir := strings.TrimSpace(item.Dir)
		if pluginID == "" || dir == "" {
			continue
		}
		seed, err := loadTemplateSeed(dir)
		if err != nil {
			continue
		}
		localID := strings.TrimSpace(seed.compiled.bundle.manifest.ID)
		if !pluginTemplateLocalIDPattern.MatchString(localID) {
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
