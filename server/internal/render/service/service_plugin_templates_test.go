package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/pluginsync"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func TestServiceSyncsPluginTemplatesAndUsesPluginAssetDigest(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")
	pluginTemplateDir := filepath.Join(repoRoot, "plugins", "installed", "weather-card", "templates", "card")
	writeRenderTemplateSeed(t, filepath.Join(repoRoot, "plugins", "installed", "weather-card", "templates"), "card")
	if err := os.MkdirAll(filepath.Join(pluginTemplateDir, "assets"), 0o755); err != nil {
		t.Fatalf("create plugin assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginTemplateDir, "assets", "token.txt"), []byte("one"), 0o644); err != nil {
		t.Fatalf("write plugin asset: %v", err)
	}

	runner := &fakeRunner{}
	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         filepath.Join(t.TempDir(), "render-output"),
		Store:              openRenderTestStore(t),
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	if err := service.SyncPluginTemplates(context.Background(), []renderplugins.Source{{
		PluginID: "weather-card",
		Dir:      pluginTemplateDir,
	}}); err != nil {
		t.Fatalf("SyncPluginTemplates: %v", err)
	}

	items, err := service.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	var found bool
	for _, item := range items {
		if item.ID == "plugin.weather-card.card" && item.Source.Type == "plugin" && item.Source.PluginID == "weather-card" && item.Source.LocalID == "card" {
			found = true
		}
	}
	if !found {
		t.Fatalf("plugin template not listed with Source: %#v", items)
	}

	request := Request{
		Template: "plugin.weather-card.card",
		Output:   "png",
		Data: map[string]any{
			"title": "天气",
		},
	}
	first, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("first Render: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginTemplateDir, "assets", "token.txt"), []byte("two"), 0o644); err != nil {
		t.Fatalf("update plugin asset: %v", err)
	}
	second, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if first.CacheKey == second.CacheKey {
		t.Fatalf("expected plugin asset change to alter cache key")
	}
	if runner.callCount() != 2 {
		t.Fatalf("runner call count = %d, want 2", runner.callCount())
	}
}

func TestValidatePluginTemplateSourcesRejectsEscapedTemplateFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	templateDir := filepath.Join(root, "templates", "card")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.HTML"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write outside HTML: %v", err)
	}
	files := map[string]string{
		"template.json": `{
  "id": "card",
  "version": "1",
  "entry_html": "../outside.HTML",
  "stylesheet": "styles.css",
  "input_schema": "input.Schema.json",
  "width": 320,
  "height": 240
}`,
		"styles.css":        "body { margin: 0; }",
		"input.Schema.json": `{"type":"object"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template file %s: %v", name, err)
		}
	}

	err := renderplugins.ValidateSources([]renderplugins.Source{{
		PluginID: "weather-card",
		Dir:      templateDir,
	}})
	if err == nil {
		t.Fatal("expected escaped template file to be rejected")
	}
}

func TestValidatePluginTemplateSourcesRejectsUnsafeLocalID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	templateDir := filepath.Join(root, "templates", "card")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	files := map[string]string{
		"template.json": `{
  "id": "card/nested",
  "version": "1",
  "entry_html": "template.HTML",
  "stylesheet": "styles.css",
  "input_schema": "input.Schema.json",
  "width": 320,
  "height": 240
}`,
		"template.HTML":     "<html><body>{{ .title }}</body></html>",
		"styles.css":        "body { margin: 0; }",
		"input.Schema.json": `{"type":"object"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template file %s: %v", name, err)
		}
	}

	err := renderplugins.ValidateSources([]renderplugins.Source{{
		PluginID: "weather-card",
		Dir:      templateDir,
	}})
	if err == nil {
		t.Fatal("expected unsafe local template id to be rejected")
	}
}

func TestServiceRejectsTemplateSourceConflicts(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "plugin.weather-card.card")
	pluginTemplateDir := filepath.Join(repoRoot, "plugins", "installed", "weather-card", "templates", "card")
	writeRenderTemplateSeed(t, filepath.Join(repoRoot, "plugins", "installed", "weather-card", "templates"), "card")

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         filepath.Join(t.TempDir(), "render-output"),
		Store:              openRenderTestStore(t),
		Runner:             &fakeRunner{},
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	err = service.SyncPluginTemplates(context.Background(), []renderplugins.Source{{
		PluginID: "weather-card",
		Dir:      pluginTemplateDir,
	}})
	if err == nil {
		t.Fatal("expected source conflict to be rejected")
	}
}

func TestServiceRemovePluginTemplatesKeepsArtifacts(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginTemplateDir := filepath.Join(repoRoot, "plugins", "installed", "weather-card", "templates", "card")
	writeRenderTemplateSeed(t, filepath.Join(repoRoot, "plugins", "installed", "weather-card", "templates"), "card")

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         filepath.Join(t.TempDir(), "render-output"),
		Store:              openRenderTestStore(t),
		Runner:             &fakeRunner{},
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	if err := service.SyncPluginTemplates(context.Background(), []renderplugins.Source{{
		PluginID: "weather-card",
		Dir:      pluginTemplateDir,
	}}); err != nil {
		t.Fatalf("SyncPluginTemplates: %v", err)
	}

	result, err := service.Render(context.Background(), Request{
		Template: "plugin.weather-card.card",
		Output:   "png",
		Data: map[string]any{
			"title": "天气",
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if err := service.RemovePluginTemplates(context.Background(), "weather-card"); err != nil {
		t.Fatalf("RemovePluginTemplates: %v", err)
	}

	items, err := service.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	for _, item := range items {
		if item.ID == "plugin.weather-card.card" {
			t.Fatalf("removed plugin template still listed: %#v", items)
		}
	}
	if _, err := service.LookupArtifact(result.ArtifactID); err != nil {
		t.Fatalf("LookupArtifact after template removal: %v", err)
	}
}

func TestServiceResolvePluginTemplateChecksDottedPluginIDOwner(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginTemplateDir := filepath.Join(repoRoot, "plugins", "installed", "com.weather", "templates", "card")
	writeRenderTemplateSeed(t, filepath.Join(repoRoot, "plugins", "installed", "com.weather", "templates"), "card")

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         filepath.Join(t.TempDir(), "render-output"),
		Store:              openRenderTestStore(t),
		Runner:             &fakeRunner{},
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	if err := service.SyncPluginTemplates(context.Background(), []renderplugins.Source{{
		PluginID: "com.weather",
		Dir:      pluginTemplateDir,
	}}); err != nil {
		t.Fatalf("SyncPluginTemplates: %v", err)
	}

	resolved, err := service.ResolvePluginTemplate(context.Background(), "com.weather", "plugin.com.weather.card")
	if err != nil {
		t.Fatalf("ResolvePluginTemplate owner failed: %v", err)
	}
	if resolved != "plugin.com.weather.card" {
		t.Fatalf("resolved = %q, want plugin.com.weather.card", resolved)
	}

	_, err = service.ResolvePluginTemplate(context.Background(), "com", "plugin.com.weather.card")
	var renderErr *rendertemplates.Error
	if !errors.As(err, &renderErr) || renderErr.Code != "plugin.capability_violation" {
		t.Fatalf("expected plugin.capability_violation, got %v", err)
	}
}
