package service

import (
	"context"
	"fmt"

	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/plugins"
)

func (s *Service) SyncPluginTemplates(ctx context.Context, sources []renderplugins.Source) error {
	if s == nil {
		return nil
	}

	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()

	prepared, err := renderplugins.PrepareSync(sources)
	if err != nil {
		return err
	}
	for _, item := range prepared.Templates {
		if err := s.syncTemplateSeed(ctx, item.TemplateID, item.Seed, item.SourceInfo, item.Dir, item.ResourceRoot); err != nil {
			return fmt.Errorf("sync plugin render template %s/%s: %w", item.PluginID, item.LocalID, err)
		}
	}

	for pluginID, keepIDs := range prepared.KeepByPlugin {
		if err := s.templateRepo.RemovePluginTemplatesExcept(ctx, pluginID, keepIDs); err != nil {
			return err
		}
	}
	if err := s.templateRepo.RemovePluginTemplatesNotIn(ctx, prepared.ActivePluginIDs); err != nil {
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

	prefix := renderplugins.Prefix(pluginID)
	s.templateRoots.RemovePrefix(prefix)
	return nil
}
