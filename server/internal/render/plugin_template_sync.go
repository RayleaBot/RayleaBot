package render

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) SyncPluginTemplates(ctx context.Context, sources []PluginTemplateSource) error {
	if s == nil {
		return nil
	}

	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()

	keepByPlugin := map[string][]string{}
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
		seed.source.ManifestJSON["id"] = templateID
		seed.compiled.bundle.manifest.ID = templateID
		seed.compiled.bundle.normalizedManifest["id"] = templateID
		seed.compiled.bundle.source.ManifestJSON["id"] = templateID
		seed.compiled.bundle.digest = digestTemplateSource(seed.compiled.bundle.source)
		seed.source = seed.compiled.bundle.source
		if err := s.syncTemplateSeed(ctx, templateID, seed, TemplateSourceInfo{
			Type:     "plugin",
			PluginID: pluginID,
			LocalID:  localID,
		}, dir, resourceRoot); err != nil {
			return fmt.Errorf("sync plugin render template %s/%s: %w", pluginID, localID, err)
		}
		keepByPlugin[pluginID] = append(keepByPlugin[pluginID], templateID)
	}

	for pluginID, keepIDs := range keepByPlugin {
		if err := s.templateRepo.RemovePluginTemplatesExcept(ctx, pluginID, keepIDs); err != nil {
			return err
		}
	}
	activePluginIDs := make([]string, 0, len(keepByPlugin))
	for pluginID := range keepByPlugin {
		activePluginIDs = append(activePluginIDs, pluginID)
	}
	if err := s.templateRepo.RemovePluginTemplatesNotIn(ctx, activePluginIDs); err != nil {
		return err
	}
	return nil
}

func (s *Service) RemovePluginTemplates(ctx context.Context, pluginID string) error {
	if s == nil {
		return nil
	}
	if err := s.templateRepo.RemovePluginTemplatesExcept(ctx, pluginID, nil); err != nil {
		return err
	}

	prefix := "plugin." + strings.TrimSpace(pluginID) + "."
	s.mu.Lock()
	defer s.mu.Unlock()
	for templateID := range s.templateRoots {
		if strings.HasPrefix(templateID, prefix) {
			delete(s.templateRoots, templateID)
		}
	}
	return nil
}
