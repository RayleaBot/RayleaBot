package service

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

func (s *Service) syncTemplatesFromFiles(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()

	Seeds, err := DiscoverSeeds(s.templatesRoot, s.logger)
	if err != nil {
		return err
	}

	for _, templateID := range sortedTemplateIDs(Seeds) {
		seed := Seeds[templateID]
		templateDir := filepath.Join(s.templatesRoot, filepath.Clean(templateID))
		if err := s.syncTemplateSeed(ctx, templateID, seed, TemplateSourceInfo{Type: "system"}, templateDir, s.templatesRoot); err != nil {
			return fmt.Errorf("sync render template %s: %w", templateID, err)
		}
	}
	return nil
}
func (s *Service) syncTemplateSeed(ctx context.Context, templateID string, seed Seed, sourceInfo TemplateSourceInfo, templateDir string, resourceRoot string) error {
	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(
		templateID,
		newRevisionID(templateID, seed.Compiled.Bundle.Digest),
		seed.Compiled,
		"save",
		nil,
		savedAt,
	)
	changed, err := s.templateRepo.SyncTemplateRevision(ctx, revision, TemplateValidationStatus{
		Valid:      true,
		CheckedAt:  savedAt,
		IssueCount: 0,
	}, sourceInfo)
	if err != nil {
		return err
	}

	s.rememberTemplateRoot(templateID, templateDir, resourceRoot)
	if changed && s.logger != nil {
		s.logger.Info(
			"render template synchronized",
			"component", "render",
			"template_id", templateID,
			"revision_id", revision.RevisionID,
			"source_digest", revision.SourceDigest,
		)
	}
	return nil
}
