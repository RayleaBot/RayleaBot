package render

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

	templateSeeds, err := discoverTemplateSeeds(s.templatesRoot, s.logger)
	if err != nil {
		return err
	}

	for _, templateID := range sortedTemplateIDs(templateSeeds) {
		seed := templateSeeds[templateID]
		templateDir := filepath.Join(s.templatesRoot, filepath.Clean(templateID))
		if err := s.syncTemplateSeed(ctx, templateID, seed, TemplateSourceInfo{Type: "system"}, templateDir, s.templatesRoot); err != nil {
			return fmt.Errorf("sync render template %s: %w", templateID, err)
		}
	}
	return nil
}
func (s *Service) syncTemplateSeed(ctx context.Context, templateID string, seed templateSeed, sourceInfo TemplateSourceInfo, templateDir string, resourceRoot string) error {
	savedAt := time.Now().UTC().Format(time.RFC3339Nano)
	revision := newStoredRevision(
		templateID,
		newRevisionID(templateID, seed.compiled.bundle.digest),
		seed.compiled,
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
