package service

import (
	"log/slog"

	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/plugins"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

type TemplateDraft = rendertemplates.TemplateDraft
type TemplateValidationIssue = rendertemplates.TemplateValidationIssue
type TemplateValidationResult = rendertemplates.TemplateValidationResult
type PluginTemplateSource = renderplugins.Source
type Root = rendertemplates.Root
type SourceBundle = rendertemplates.SourceBundle
type CompiledTemplate = rendertemplates.CompiledTemplate
type Seed = rendertemplates.Seed

type templateManifest = rendertemplates.Manifest
type templateSourceBundle = rendertemplates.SourceBundle
type compiledTemplate = rendertemplates.CompiledTemplate
type templateSeed = rendertemplates.Seed

const defaultTemplatePreviewData = rendertemplates.DefaultPreviewData

func BuildSourceBundle(expectedTemplateID string, source TemplateSource) (SourceBundle, error) {
	return rendertemplates.BuildSourceBundle(expectedTemplateID, source)
}

func CompileBundle(bundle SourceBundle) (*CompiledTemplate, []TemplateValidationIssue, error) {
	return rendertemplates.CompileBundle(bundle)
}

func DiscoverSeeds(root string, logger *slog.Logger) (map[string]Seed, error) {
	return rendertemplates.DiscoverSeeds(root, logger)
}

func ResourceDigest(templateDir string) string {
	return rendertemplates.ResourceDigest(templateDir)
}

func ResolveAssetPath(root Root, relativePath string) (string, error) {
	return rendertemplates.ResolveAssetPath(root, relativePath)
}

func ManagedSourcePaths(templateDir string, files TemplateFiles) []string {
	return rendertemplates.ManagedSourcePaths(templateDir, files)
}

func sameFilePath(left, right string) bool {
	return rendertemplates.SameFilePath(left, right)
}

func sortedTemplateIDs(seeds map[string]Seed) []string {
	return rendertemplates.SortedIDs(seeds)
}
