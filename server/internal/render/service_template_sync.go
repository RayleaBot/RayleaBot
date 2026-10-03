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
	seeds, err := discoverSeeds(s.repoRoot, s.templatesRoot, s.logger, s.templateCompiler.compileSystem)
	if err != nil {
		return err
	}
	ids := SortedIDs(seeds)
	updated := 0
	for _, id := range ids {
		changed, err := s.syncTemplateSeed(ctx, id, seeds[id], TemplateSourceInfo{Type: "system"}, filepath.Join(s.templatesRoot, id), s.templatesRoot)
		if err != nil {
			return fmt.Errorf("sync render template %s: %w", id, err)
		}
		if changed {
			updated++
		}
	}
	if err := s.removeSystemTemplatesExcept(ctx, ids); err != nil {
		return err
	}
	s.logTemplateSync(updated, "system")
	return nil
}

func (s *Service) removeSystemTemplatesExcept(ctx context.Context, ids []string) error {
	s.templateCompiler.mu.Lock()
	defer s.templateCompiler.mu.Unlock()
	if err := s.templateRepo.RemoveSystemTemplatesExcept(ctx, ids); err != nil {
		return err
	}
	keep := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		keep[id] = struct{}{}
	}
	s.templateCompiler.removeExceptLocked("system", "", keep)
	return nil
}

func (s *Service) syncTemplateSeed(ctx context.Context, id string, seed Seed, owner TemplateSourceInfo, templateDir, resourceRoot string) (bool, error) {
	changed, err := s.templateRepo.SyncTemplate(ctx, currentTemplate{
		ID: id, SourceDigest: seed.Compiled.Bundle.Digest, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Source: seed.Compiled.Bundle.Source, Owner: owner,
	})
	if err != nil {
		s.templateCompiler.mu.Lock()
		delete(s.templateCompiler.entries, id)
		s.templateCompiler.mu.Unlock()
		return false, err
	}
	s.rememberTemplateRoot(id, templateDir, resourceRoot)
	if changed && s.logger != nil {
		s.logger.Debug("图片模板已更新", "component", "render", "template_id", id, "source_digest", seed.Compiled.Bundle.Digest)
	}
	return changed, nil
}

func (s *Service) logTemplateSync(updated int, sourceType string) {
	if updated > 0 && s.logger != nil {
		s.logger.Info(fmt.Sprintf("图片模板已更新，共 %d 个", updated), "component", "render", "updated_count", updated, "source_type", sourceType)
	}
}
