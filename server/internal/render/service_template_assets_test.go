package render

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLookupTemplateAssetRespectsSystemResourceRoot(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")
	writeRenderTemplateSeed(t, templatesRoot, "shared.card")
	assetPath := filepath.Join(templatesRoot, "shared.card", "assets", "logo.txt")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
		t.Fatalf("create shared asset dir: %v", err)
	}
	if err := os.WriteFile(assetPath, []byte("asset"), 0o644); err != nil {
		t.Fatalf("write shared asset: %v", err)
	}

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

	asset, err := service.LookupTemplateAsset(context.Background(), "help.menu", "../shared.card/assets/logo.txt")
	if err != nil {
		t.Fatalf("LookupTemplateAsset shared asset: %v", err)
	}
	if asset.Path != assetPath {
		t.Fatalf("asset path = %q, want %q", asset.Path, assetPath)
	}

	for _, path := range []string{"../../outside.txt", "template.html", "styles.css", "input.schema.json", "preview.json", "missing.png"} {
		_, err := service.LookupTemplateAsset(context.Background(), "help.menu", path)
		var renderErr *Error
		if !errors.As(err, &renderErr) || renderErr.Code != "platform.resource_missing" {
			t.Fatalf("LookupTemplateAsset(%q) error = %v, want platform.resource_missing", path, err)
		}
	}
}

func TestLookupTemplateAssetRejectsRegisteredSourceFiles(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	templateDir := filepath.Join(templatesRoot, "custom.card")
	if err := os.MkdirAll(filepath.Join(templateDir, "assets"), 0o755); err != nil {
		t.Fatalf("create custom template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "template.json"), []byte(`{"id":"custom.card","version":"1","entry_html":"views/card.gohtml","stylesheet":"css/card.main.css","input_schema":"schema/input.json","width":960,"height":640}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	for path, content := range map[string]string{
		"views/card.gohtml": "<html><body>{{ .title }}</body></html>",
		"css/card.main.css": "body { margin: 0; }",
		"schema/input.json": `{"type":"object","properties":{"title":{"type":"string"}}}`,
		"preview.json":      `{"title":"custom"}`,
		"assets/logo.txt":   "asset",
	} {
		target := filepath.Join(templateDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create %s dir: %v", path, err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

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

	asset, err := service.LookupTemplateAsset(context.Background(), "custom.card", "assets/logo.txt")
	if err != nil {
		t.Fatalf("LookupTemplateAsset allowed asset: %v", err)
	}
	if asset.Path != filepath.Join(templateDir, "assets", "logo.txt") {
		t.Fatalf("asset path = %q", asset.Path)
	}

	for _, path := range []string{"template.json", "views/card.gohtml", "css/card.main.css", "schema/input.json", "preview.json"} {
		_, err := service.LookupTemplateAsset(context.Background(), "custom.card", path)
		var renderErr *Error
		if !errors.As(err, &renderErr) || renderErr.Code != "platform.resource_missing" {
			t.Fatalf("LookupTemplateAsset(%q) error = %v, want platform.resource_missing", path, err)
		}
	}
}

func TestLookupTemplateAssetRespectsPluginPackageRoot(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	pluginRoot := filepath.Join(repoRoot, "plugins", "installed", "weather-card")
	pluginTemplateDir := filepath.Join(pluginRoot, "templates", "card")
	writeRenderTemplateSeed(t, filepath.Join(pluginRoot, "templates"), "card")
	assetPath := filepath.Join(pluginRoot, "assets", "icon.txt")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
		t.Fatalf("create plugin asset dir: %v", err)
	}
	if err := os.WriteFile(assetPath, []byte("plugin asset"), 0o644); err != nil {
		t.Fatalf("write plugin asset: %v", err)
	}

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
	if err := service.SyncPluginTemplates(context.Background(), []PluginTemplateSource{{
		PluginID:     "weather-card",
		Dir:          pluginTemplateDir,
		ResourceRoot: pluginRoot,
	}}); err != nil {
		t.Fatalf("SyncPluginTemplates: %v", err)
	}

	asset, err := service.LookupTemplateAsset(context.Background(), "plugin.weather-card.card", "../../assets/icon.txt")
	if err != nil {
		t.Fatalf("LookupTemplateAsset plugin asset: %v", err)
	}
	if asset.Path != assetPath {
		t.Fatalf("asset path = %q, want %q", asset.Path, assetPath)
	}

	_, err = service.LookupTemplateAsset(context.Background(), "plugin.weather-card.card", "../../../outside.txt")
	var renderErr *Error
	if !errors.As(err, &renderErr) || renderErr.Code != "platform.resource_missing" {
		t.Fatalf("expected escaped plugin path rejection, got %v", err)
	}
}
