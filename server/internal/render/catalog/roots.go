package catalog

import (
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

type Roots struct {
	mu            sync.RWMutex
	templatesRoot string
	entries       map[string]rendertemplates.Root
}

func NewRoots(templatesRoot string) *Roots {
	return &Roots{
		templatesRoot: templatesRoot,
		entries:       map[string]rendertemplates.Root{},
	}
}

func (r *Roots) Remember(templateID, templateDir, resourceRoot string) {
	if r == nil || strings.TrimSpace(templateID) == "" || strings.TrimSpace(templateDir) == "" {
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

	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[strings.TrimSpace(templateID)] = rendertemplates.Root{
		TemplateDir:  absoluteTemplateDir,
		ResourceRoot: absoluteResourceRoot,
	}
}

func (r *Roots) TemplateDir(templateID string) string {
	templateID = strings.TrimSpace(templateID)
	if r == nil {
		return filepath.Clean(templateID)
	}
	r.mu.RLock()
	if root := r.entries[templateID]; root.TemplateDir != "" {
		r.mu.RUnlock()
		return root.TemplateDir
	}
	r.mu.RUnlock()
	return filepath.Join(r.templatesRoot, filepath.Clean(templateID))
}

func (r *Roots) TemplateRoot(templateID string) rendertemplates.Root {
	templateID = strings.TrimSpace(templateID)
	if r == nil {
		return rendertemplates.Root{}
	}
	r.mu.RLock()
	root := r.entries[templateID]
	r.mu.RUnlock()
	if root.TemplateDir != "" && root.ResourceRoot != "" {
		return root
	}
	templateDir := filepath.Join(r.templatesRoot, filepath.Clean(templateID))
	return rendertemplates.Root{
		TemplateDir:  templateDir,
		ResourceRoot: r.templatesRoot,
	}
}

func (r *Roots) RemovePrefix(prefix string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for templateID := range r.entries {
		if strings.HasPrefix(templateID, prefix) {
			delete(r.entries, templateID)
		}
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
