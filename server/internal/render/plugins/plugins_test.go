package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareSyncRewritesPluginTemplateID(t *testing.T) {
	t.Parallel()

	templateDir := writeTemplateSeed(t, t.TempDir(), "card")
	prepared, err := PrepareSync([]Source{{
		PluginID: "weather-card",
		Dir:      templateDir,
	}})
	if err != nil {
		t.Fatalf("PrepareSync: %v", err)
	}
	if len(prepared.Templates) != 1 {
		t.Fatalf("prepared templates = %d, want 1", len(prepared.Templates))
	}
	item := prepared.Templates[0]
	if item.TemplateID != "plugin.weather-card.card" {
		t.Fatalf("TemplateID = %q", item.TemplateID)
	}
	if item.Seed.Compiled.Bundle.Manifest.ID != item.TemplateID {
		t.Fatalf("compiled manifest id = %q", item.Seed.Compiled.Bundle.Manifest.ID)
	}
	if item.Seed.Source.ManifestJSON["id"] != item.TemplateID {
		t.Fatalf("source manifest id = %#v", item.Seed.Source.ManifestJSON["id"])
	}
	if item.SourceInfo.Type != "plugin" || item.SourceInfo.PluginID != "weather-card" || item.SourceInfo.LocalID != "card" {
		t.Fatalf("SourceInfo = %#v", item.SourceInfo)
	}
}

func TestValidateSourcesRejectsUnsafeLocalID(t *testing.T) {
	t.Parallel()

	templateDir := writeTemplateSeed(t, t.TempDir(), "card/nested")
	err := ValidateSources([]Source{{
		PluginID: "weather-card",
		Dir:      templateDir,
	}})
	if err == nil {
		t.Fatal("expected unsafe local template id to be rejected")
	}
}

func writeTemplateSeed(t *testing.T, templatesRoot, templateID string) string {
	t.Helper()

	templateDir := filepath.Join(templatesRoot, "template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	files := map[string]string{
		"template.json": `{
  "id": "` + templateID + `",
  "version": "1",
  "entry_html": "template.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 320,
  "height": 240
}`,
		"template.html":     "<html><body>{{ .title }}</body></html>",
		"styles.css":        "body { margin: 0; }",
		"input.schema.json": `{"type":"object"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template file %s: %v", name, err)
		}
	}
	return templateDir
}
