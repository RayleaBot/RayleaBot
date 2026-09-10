package render

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

func (s *Service) syncTemplatesFromFiles(ctx context.Context) error {
	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()
	seeds, err := DiscoverSeeds(s.repoRoot, s.templatesRoot, s.logger)
	if err != nil {
		return err
	}
	ids := SortedIDs(seeds)
	for _, id := range ids {
		if err := s.syncTemplateSeed(ctx, id, seeds[id], TemplateSourceInfo{Type: "system"}, filepath.Join(s.templatesRoot, id), s.templatesRoot); err != nil {
			return fmt.Errorf("sync render template %s: %w", id, err)
		}
	}
	return s.templateRepo.RemoveSystemTemplatesExcept(ctx, ids)
}

func (s *Service) syncTemplateSeed(ctx context.Context, id string, seed Seed, owner TemplateSourceInfo, templateDir, resourceRoot string) error {
	changed, err := s.templateRepo.SyncTemplate(ctx, currentTemplate{
		ID: id, SourceDigest: seed.Compiled.Bundle.Digest, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Source: seed.Compiled.Bundle.Source, Owner: owner,
	})
	if err != nil {
		return err
	}
	s.rememberTemplateRoot(id, templateDir, resourceRoot)
	if changed && s.logger != nil {
		s.logger.Info("图片模板已更新", "component", "render", "template_id", id, "source_digest", seed.Compiled.Bundle.Digest)
	}
	return nil
}
