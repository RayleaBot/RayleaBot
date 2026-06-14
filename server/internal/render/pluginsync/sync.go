package pluginsync

import (
	"fmt"
	"sort"
	"strings"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func PrepareSync(sources []Source) (PreparedSync, error) {
	prepared := PreparedSync{
		KeepByPlugin: map[string][]string{},
	}
	seenTemplates := map[string]struct{}{}

	for _, source := range sources {
		pluginID := strings.TrimSpace(source.PluginID)
		dir := strings.TrimSpace(source.Dir)
		if pluginID == "" || dir == "" {
			continue
		}
		resourceRoot := strings.TrimSpace(source.ResourceRoot)
		if resourceRoot == "" {
			resourceRoot = dir
		}

		seed, localID, err := loadValidSeed(pluginID, dir)
		if err != nil {
			return PreparedSync{}, err
		}
		templateID := FormalID(pluginID, localID)
		if _, ok := seenTemplates[templateID]; ok {
			return PreparedSync{}, fmt.Errorf("duplicate plugin render template id %s", templateID)
		}
		seenTemplates[templateID] = struct{}{}

		seed = rewriteSeedID(seed, templateID)
		prepared.Templates = append(prepared.Templates, PreparedTemplate{
			PluginID:     pluginID,
			LocalID:      localID,
			TemplateID:   templateID,
			Dir:          dir,
			ResourceRoot: resourceRoot,
			SourceInfo: renderrepo.TemplateSourceInfo{
				Type:     "plugin",
				PluginID: pluginID,
				LocalID:  localID,
			},
			Seed: seed,
		})
		prepared.KeepByPlugin[pluginID] = append(prepared.KeepByPlugin[pluginID], templateID)
	}

	for pluginID := range prepared.KeepByPlugin {
		prepared.ActivePluginIDs = append(prepared.ActivePluginIDs, pluginID)
	}
	sort.Strings(prepared.ActivePluginIDs)
	return prepared, nil
}

func rewriteSeedID(seed rendertemplates.Seed, templateID string) rendertemplates.Seed {
	seed.Source.ManifestJSON["id"] = templateID
	seed.Compiled.Bundle.Manifest.ID = templateID
	seed.Compiled.Bundle.NormalizedManifest["id"] = templateID
	seed.Compiled.Bundle.Source.ManifestJSON["id"] = templateID
	seed.Compiled.Bundle.Digest = rendertemplates.DigestSource(seed.Compiled.Bundle.Source)
	seed.Source = seed.Compiled.Bundle.Source
	return seed
}
