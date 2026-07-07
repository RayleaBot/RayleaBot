package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

type Source struct {
	PluginID     string
	LocalID      string
	Dir          string
	ResourceRoot string
}

type PreparedTemplate struct {
	PluginID     string
	LocalID      string
	TemplateID   string
	Dir          string
	ResourceRoot string
	SourceInfo   renderrepo.TemplateSourceInfo
	Seed         Seed
}

type PreparedSync struct {
	Templates       []PreparedTemplate
	KeepByPlugin    map[string][]string
	ActivePluginIDs []string
}

func (s *Service) SyncPluginTemplates(ctx context.Context, sources []Source) error {
	if s == nil {
		return nil
	}

	s.templateSyncMu.Lock()
	defer s.templateSyncMu.Unlock()

	prepared, err := PrepareSync(sources)
	if err != nil {
		return err
	}
	for _, item := range prepared.Templates {
		if err := s.syncTemplateSeed(ctx, item.TemplateID, item.Seed, item.SourceInfo, item.Dir, item.ResourceRoot); err != nil {
			return fmt.Errorf("sync plugin render template %s/%s: %w", item.PluginID, item.LocalID, err)
		}
	}

	for pluginID, keepIDs := range prepared.KeepByPlugin {
		if err := s.templateRepo.RemovePluginTemplatesExcept(ctx, pluginID, keepIDs); err != nil {
			return err
		}
	}
	if err := s.templateRepo.RemovePluginTemplatesNotIn(ctx, prepared.ActivePluginIDs); err != nil {
		return err
	}
	return nil
}

func (s *Service) RemovePluginTemplates(ctx context.Context, pluginID string) error {
	if s == nil {
		return nil
	}
	if err := s.templateRepo.RemovePluginTemplatesExcept(ctx, pluginID, nil); err != nil {
		return err
	}

	prefix := Prefix(pluginID)
	s.templateRoots.RemovePrefix(prefix)
	return nil
}

type PluginTemplateDeclaration struct {
	PluginID          string
	Path              string
	PackageRootPath   string
	Valid             bool
	RegistrationState string
}

var localIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func FormalID(pluginID, localID string) string {
	pluginID = strings.TrimSpace(pluginID)
	localID = strings.Trim(filepath.ToSlash(strings.TrimSpace(localID)), "/")
	if pluginID == "" || localID == "" {
		return ""
	}
	return "plugin." + pluginID + "." + localID
}

func Prefix(pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return ""
	}
	return "plugin." + pluginID + "."
}

func ParseFormalID(templateID string) (string, string, bool) {
	templateID = strings.TrimSpace(templateID)
	const prefix = "plugin."
	if !strings.HasPrefix(templateID, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(templateID, prefix)
	separator := strings.LastIndex(remainder, ".")
	if separator <= 0 || separator == len(remainder)-1 {
		return "", "", false
	}
	pluginID := strings.TrimSpace(remainder[:separator])
	localID := strings.TrimSpace(remainder[separator+1:])
	if pluginID == "" || !IsValidLocalID(localID) {
		return "", "", false
	}
	return pluginID, localID, true
}

func IsValidLocalID(localID string) bool {
	return localIDPattern.MatchString(strings.TrimSpace(localID))
}

func (s *Service) SyncPluginTemplateDeclarations(ctx context.Context, declarations []PluginTemplateDeclaration) error {
	return s.SyncPluginTemplates(ctx, pluginTemplateSourcesFromDeclarations(declarations))
}

func ValidatePluginTemplateDeclarations(declarations []PluginTemplateDeclaration) error {
	sources := make([]Source, 0, len(declarations))
	for _, declaration := range declarations {
		source, ok := pluginTemplateSource(declaration)
		if !ok {
			return fmt.Errorf("plugin render template path %q is invalid", declaration.Path)
		}
		sources = append(sources, source)
	}
	return ValidateSources(sources)
}

func pluginTemplateSourcesFromDeclarations(declarations []PluginTemplateDeclaration) []Source {
	sources := make([]Source, 0, len(declarations))
	seen := map[string]struct{}{}
	for _, declaration := range declarations {
		if !declaration.Valid || declaration.RegistrationState != "installed" {
			continue
		}
		source, ok := pluginTemplateSource(declaration)
		if !ok {
			continue
		}
		key := source.PluginID + "\x00" + source.Dir
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, source)
	}
	return SourcesFromManifests(sources)
}

func pluginTemplateSource(declaration PluginTemplateDeclaration) (Source, bool) {
	pluginID := strings.TrimSpace(declaration.PluginID)
	packageRoot := strings.TrimSpace(declaration.PackageRootPath)
	relativePath := strings.TrimSpace(declaration.Path)
	if pluginID == "" || packageRoot == "" || relativePath == "" || filepath.IsAbs(relativePath) {
		return Source{}, false
	}
	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return Source{}, false
	}
	absoluteRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return Source{}, false
	}
	candidate := filepath.Join(absoluteRoot, cleanRelative)
	relativeToRoot, err := filepath.Rel(absoluteRoot, candidate)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return Source{}, false
	}
	return Source{
		PluginID:     pluginID,
		Dir:          candidate,
		ResourceRoot: packageRoot,
	}, true
}

func ValidateSources(sources []Source) error {
	seenTemplates := map[string]struct{}{}
	for _, source := range sources {
		pluginID := strings.TrimSpace(source.PluginID)
		dir := strings.TrimSpace(source.Dir)
		if pluginID == "" || dir == "" {
			return fmt.Errorf("plugin render template declaration is incomplete")
		}
		_, localID, err := loadValidSeed(pluginID, dir)
		if err != nil {
			return err
		}
		templateID := FormalID(pluginID, localID)
		if _, ok := seenTemplates[templateID]; ok {
			return fmt.Errorf("duplicate plugin render template id %s", templateID)
		}
		seenTemplates[templateID] = struct{}{}
	}
	return nil
}

func SourcesFromManifests(items []Source) []Source {
	sources := make([]Source, 0, len(items))
	for _, item := range items {
		pluginID := strings.TrimSpace(item.PluginID)
		dir := strings.TrimSpace(item.Dir)
		if pluginID == "" || dir == "" {
			continue
		}
		_, localID, err := loadValidSeed(pluginID, dir)
		if err != nil {
			continue
		}
		item.PluginID = pluginID
		item.LocalID = localID
		item.Dir = dir
		item.ResourceRoot = strings.TrimSpace(item.ResourceRoot)
		sources = append(sources, item)
	}
	return sources
}

func loadValidSeed(pluginID, dir string) (Seed, string, error) {
	seed, err := LoadSeed(dir)
	if err != nil {
		return Seed{}, "", fmt.Errorf("load plugin render template %s: %w", pluginID, err)
	}
	localID := strings.TrimSpace(seed.Compiled.Bundle.Manifest.ID)
	if !IsValidLocalID(localID) {
		return Seed{}, "", fmt.Errorf("plugin render template %s has invalid local id %q", pluginID, localID)
	}
	return seed, localID, nil
}

func (s *Service) ResolvePluginTemplate(ctx context.Context, pluginID, requested string) (string, error) {
	if s == nil {
		return "", &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	pluginID = strings.TrimSpace(pluginID)
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", &Error{Code: "platform.invalid_request", Message: "render template is required"}
	}
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return "", err
	}

	if strings.HasPrefix(requested, "plugin.") {
		ownerPluginID, _, ok := ParseFormalID(requested)
		if !ok || pluginID == "" || ownerPluginID != pluginID {
			return "", &Error{
				Code:    "plugin.capability_violation",
				Message: "plugin render template belongs to another plugin",
			}
		}
		detail, err := s.templateRepo.GetTemplateDetail(ctx, requested)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return requested, nil
			}
			return "", fmt.Errorf("get plugin render template %s: %w", requested, err)
		}
		if detail.Source.Type == "plugin" && detail.Source.PluginID != pluginID {
			return "", &Error{
				Code:    "plugin.capability_violation",
				Message: "plugin render template belongs to another plugin",
			}
		}
		return requested, nil
	}

	formalID := FormalID(pluginID, requested)
	if detail, err := s.templateRepo.GetTemplateDetail(ctx, formalID); err == nil {
		if detail.Source.Type == "plugin" && detail.Source.PluginID == pluginID && detail.Source.LocalID == requested {
			return formalID, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("get plugin render template %s: %w", formalID, err)
	}

	return requested, nil
}

func PrepareSync(sources []Source) (PreparedSync, error) {
	prepared := PreparedSync{
		KeepByPlugin: map[string][]string{},
	}
	seenTemplates := map[string]struct{}{}

	for _, source := range sources {
		pluginID := strings.TrimSpace(source.PluginID)
		dir := strings.TrimSpace(source.Dir)
		if pluginID == "" || dir == "" {
			continue
		}
		resourceRoot := strings.TrimSpace(source.ResourceRoot)
		if resourceRoot == "" {
			resourceRoot = dir
		}

		seed, localID, err := loadValidSeed(pluginID, dir)
		if err != nil {
			return PreparedSync{}, err
		}
		templateID := FormalID(pluginID, localID)
		if _, ok := seenTemplates[templateID]; ok {
			return PreparedSync{}, fmt.Errorf("duplicate plugin render template id %s", templateID)
		}
		seenTemplates[templateID] = struct{}{}

		seed = rewriteSeedID(seed, templateID)
		prepared.Templates = append(prepared.Templates, PreparedTemplate{
			PluginID:     pluginID,
			LocalID:      localID,
			TemplateID:   templateID,
			Dir:          dir,
			ResourceRoot: resourceRoot,
			SourceInfo: renderrepo.TemplateSourceInfo{
				Type:     "plugin",
				PluginID: pluginID,
				LocalID:  localID,
			},
			Seed: seed,
		})
		prepared.KeepByPlugin[pluginID] = append(prepared.KeepByPlugin[pluginID], templateID)
	}

	for pluginID := range prepared.KeepByPlugin {
		prepared.ActivePluginIDs = append(prepared.ActivePluginIDs, pluginID)
	}
	sort.Strings(prepared.ActivePluginIDs)
	return prepared, nil
}

func rewriteSeedID(seed Seed, templateID string) Seed {
	seed.Source.ManifestJSON["id"] = templateID
	seed.Compiled.Bundle.Manifest.ID = templateID
	seed.Compiled.Bundle.NormalizedManifest["id"] = templateID
	seed.Compiled.Bundle.Source.ManifestJSON["id"] = templateID
	seed.Compiled.Bundle.Digest = DigestSource(seed.Compiled.Bundle.Source)
	seed.Source = seed.Compiled.Bundle.Source
	return seed
}
