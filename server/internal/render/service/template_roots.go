package service

import (
	rendercatalog "github.com/RayleaBot/RayleaBot/server/internal/render/catalog"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) rememberTemplateRoot(templateID, templateDir, resourceRoot string) {
	if s == nil {
		return
	}
	s.templateRoots.Remember(templateID, templateDir, resourceRoot)
}

func (s *Service) templateDirFor(templateID string) string {
	if s == nil {
		return ""
	}
	return s.templateRoots.TemplateDir(templateID)
}

func (s *Service) templateRootFor(templateID string) rendertemplates.Root {
	if s == nil {
		return rendertemplates.Root{}
	}
	return s.templateRoots.TemplateRoot(templateID)
}

func BaseURL(templateDir string) string {
	return rendercatalog.BaseURL(templateDir)
}
