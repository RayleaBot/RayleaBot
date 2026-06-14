package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	rendercatalog "github.com/RayleaBot/RayleaBot/server/internal/render/catalog"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) LookupTemplateAsset(ctx context.Context, templateID string, relativePath string) (TemplateAsset, error) {
	if s == nil {
		return TemplateAsset{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return TemplateAsset{}, err
	}

	templateID = strings.TrimSpace(templateID)
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return TemplateAsset{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return TemplateAsset{}, err
	}

	root := s.templateRootFor(templateID)
	if root.TemplateDir == "" || root.ResourceRoot == "" {
		return TemplateAsset{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	assetPath, err := rendertemplates.ResolveAssetPath(root, relativePath)
	if err != nil {
		return TemplateAsset{}, err
	}
	isSourcePath, err := rendercatalog.IsManagedTemplateSourcePath(ctx, s.templateRepo, s.templateRoots, assetPath)
	if err != nil {
		return TemplateAsset{}, err
	}
	if isSourcePath {
		return TemplateAsset{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	info, err := os.Stat(assetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return TemplateAsset{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render template asset was not found", Err: err}
		}
		return TemplateAsset{}, fmt.Errorf("inspect render template asset %s: %w", assetPath, err)
	}
	if info.IsDir() {
		return TemplateAsset{}, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}

	return TemplateAsset{Path: assetPath}, nil
}
