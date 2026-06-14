package service

import (
	"net/url"
	"path/filepath"
	"strings"
)

func (s *Service) rememberTemplateRoot(templateID, templateDir, resourceRoot string) {
	if s == nil || strings.TrimSpace(templateID) == "" || strings.TrimSpace(templateDir) == "" {
		return
	}
	absoluteTemplateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return
	}
	if strings.TrimSpace(resourceRoot) == "" {
		resourceRoot = templateDir
	}
	absoluteResourceRoot, err := filepath.Abs(resourceRoot)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templateRoots[strings.TrimSpace(templateID)] = templateRoot{
		TemplateDir:  absoluteTemplateDir,
		ResourceRoot: absoluteResourceRoot,
	}
}

func (s *Service) templateDirFor(templateID string) string {
	if s == nil {
		return ""
	}
	templateID = strings.TrimSpace(templateID)
	s.mu.RLock()
	if root := s.templateRoots[templateID]; root.TemplateDir != "" {
		s.mu.RUnlock()
		return root.TemplateDir
	}
	s.mu.RUnlock()
	return filepath.Join(s.templatesRoot, filepath.Clean(templateID))
}

func (s *Service) templateRootFor(templateID string) templateRoot {
	if s == nil {
		return templateRoot{}
	}
	templateID = strings.TrimSpace(templateID)
	s.mu.RLock()
	root := s.templateRoots[templateID]
	s.mu.RUnlock()
	if root.TemplateDir != "" && root.ResourceRoot != "" {
		return root
	}
	templateDir := filepath.Join(s.templatesRoot, filepath.Clean(templateID))
	return templateRoot{
		TemplateDir:  templateDir,
		ResourceRoot: s.templatesRoot,
	}
}

func BaseURL(templateDir string) string {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil || templateDir == "" {
		return ""
	}
	path := filepath.ToSlash(templateDir)
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return (&url.URL{
		Scheme: "file",
		Path:   path,
	}).String()
}
