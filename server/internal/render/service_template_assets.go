package render

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) LookupTemplateAsset(ctx context.Context, templateID string, relativePath string) (TemplateAsset, error) {
	if s == nil {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return TemplateAsset{}, err
	}

	templateID = strings.TrimSpace(templateID)
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return TemplateAsset{}, err
	}

	root := s.templateRootFor(templateID)
	if root.TemplateDir == "" || root.ResourceRoot == "" {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	assetPath, err := resolveTemplateAssetPath(root, relativePath)
	if err != nil {
		return TemplateAsset{}, err
	}
	isSourcePath, err := s.isManagedTemplateSourcePath(ctx, assetPath)
	if err != nil {
		return TemplateAsset{}, err
	}
	if isSourcePath {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	info, err := os.Stat(assetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found", Err: err}
		}
		return TemplateAsset{}, fmt.Errorf("inspect render template asset %s: %w", assetPath, err)
	}
	if info.IsDir() {
		return TemplateAsset{}, &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}

	return TemplateAsset{Path: assetPath}, nil
}

func (s *Service) isManagedTemplateSourcePath(ctx context.Context, candidate string) (bool, error) {
	absoluteCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false, err
	}
	items, err := s.templateRepo.ListTemplateSummaries(ctx)
	if err != nil {
		return false, fmt.Errorf("list render templates for asset lookup: %w", err)
	}

	for _, item := range items {
		detail, err := s.templateRepo.GetTemplateDetail(ctx, item.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return false, fmt.Errorf("get render template %s for asset lookup: %w", item.ID, err)
		}

		root := s.templateRootFor(item.ID)
		if root.TemplateDir == "" {
			continue
		}
		for _, sourcePath := range managedTemplateSourcePaths(root.TemplateDir, detail.Files) {
			if sameFilePath(absoluteCandidate, sourcePath) {
				return true, nil
			}
		}
	}
	return false, nil
}
