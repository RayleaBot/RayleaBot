package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

type TemplateRepository interface {
	ListTemplateSummaries(ctx context.Context) ([]renderrepo.TemplateSummary, error)
	GetTemplateDetail(ctx context.Context, templateID string) (renderrepo.TemplateDetail, error)
}

func IsManagedTemplateSourcePath(ctx context.Context, repository TemplateRepository, roots *Roots, candidate string) (bool, error) {
	absoluteCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false, err
	}
	items, err := repository.ListTemplateSummaries(ctx)
	if err != nil {
		return false, fmt.Errorf("list render templates for asset lookup: %w", err)
	}

	for _, item := range items {
		detail, err := repository.GetTemplateDetail(ctx, item.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return false, fmt.Errorf("get render template %s for asset lookup: %w", item.ID, err)
		}

		root := roots.TemplateRoot(item.ID)
		if root.TemplateDir == "" {
			continue
		}
		for _, sourcePath := range rendertemplates.ManagedSourcePaths(root.TemplateDir, detail.Files) {
			if rendertemplates.SameFilePath(absoluteCandidate, sourcePath) {
				return true, nil
			}
		}
	}
	return false, nil
}
