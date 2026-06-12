package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

var testPNGBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")

type fakeRunner struct {
	mu      sync.Mutex
	calls   int
	closes  int
	delay   time.Duration
	waitCh  chan struct{}
	content []byte
	err     error
	docs    []Document
}

func (f *fakeRunner) Render(ctx context.Context, doc Document) ([]byte, error) {
	f.mu.Lock()
	f.calls++
	f.docs = append(f.docs, doc)
	delay := f.delay
	waitCh := f.waitCh
	content := append([]byte(nil), f.content...)
	err := f.err
	f.mu.Unlock()

	if waitCh != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-waitCh:
		}
	}

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		content = append([]byte(nil), testPNGBytes...)
	}
	if doc.Output == "jpeg" {
		return []byte{0xff, 0xd8, 0xff, 0xd9}, nil
	}
	return content, nil
}

func (f *fakeRunner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeRunner) lastDocument() (Document, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.docs) == 0 {
		return Document{}, false
	}
	return f.docs[len(f.docs)-1], true
}

type fakeCloseableRunner struct {
	fakeRunner
}

func (f *fakeCloseableRunner) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closes++
	return nil
}

func (f *fakeCloseableRunner) closeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closes
}

func TestNewServiceSkipsInvalidTemplateDirectories(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

	previewDir := filepath.Join(templatesRoot, ".preview")
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatalf("create preview dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(previewDir, "help.menu.html"), []byte("<html>preview</html>"), 0o644); err != nil {
		t.Fatalf("write preview html: %v", err)
	}

	var logs bytes.Buffer
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
		Logger:             slog.New(slog.NewTextHandler(&logs, nil)),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	items, err := service.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(items) != 1 || items[0].ID != "help.menu" {
		t.Fatalf("unexpected templates: %#v", items)
	}

	logText := logs.String()
	if !strings.Contains(logText, "render template skipped") || !strings.Contains(logText, ".preview") {
		t.Fatalf("expected skipped template warning, got %q", logText)
	}
}

func TestServiceCloseClosesRunner(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(repoRoot, "templates"), "help.menu")
	runner := &fakeCloseableRunner{}
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

	if err := service.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if runner.closeCount() != 1 {
		t.Fatalf("runner close count = %d, want 1", runner.closeCount())
	}
}

func TestRefreshBrowserPathReplacesAndClosesDefaultChromiumRunner(t *testing.T) {
	t.Parallel()

	oldChromiumRunner := NewChromiumRunner(ChromiumOptions{BrowserPath: "old-browser"}).(*chromiumRunner)
	oldChromiumRunner.browserCtx = context.Background()
	oldChromiumRunner.cancelBrowser = func() {}
	oldChromiumRunner.cancelAllocator = func() {}
	service := &Service{
		runner:      oldChromiumRunner,
		browserArgs: []string{"--disable-dev-shm-usage"},
		workerSem:   make(chan struct{}, 1),
	}
	oldRunner := service.runner

	service.RefreshBrowserPath("new-browser")

	if service.runner == oldRunner {
		t.Fatal("expected default chromium runner to be replaced")
	}
	replaced, ok := service.runner.(*chromiumRunner)
	if !ok {
		t.Fatalf("expected chromium runner, got %T", service.runner)
	}
	if replaced.browserPath != "new-browser" {
		t.Fatalf("browser path = %q, want new-browser", replaced.browserPath)
	}
	old, ok := oldRunner.(*chromiumRunner)
	if !ok {
		t.Fatalf("old runner type = %T, want chromium runner", oldRunner)
	}
	if old.browserCtx != nil || old.cancelBrowser != nil || old.cancelAllocator != nil {
		t.Fatalf("old chromium runner was not closed")
	}
}

func TestRefreshBrowserPathKeepsInjectedRunner(t *testing.T) {
	t.Parallel()

	runner := &fakeCloseableRunner{}
	service := &Service{runner: runner}

	service.RefreshBrowserPath("new-browser")

	if service.runner != runner {
		t.Fatal("expected injected runner to remain unchanged")
	}
	if runner.closeCount() != 0 {
		t.Fatalf("injected runner close count = %d, want 0", runner.closeCount())
	}
}

func TestRenderInjectsFooterWithDevelopmentSystemContext(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")
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

	if _, err := service.Render(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title":         "帮助",
			"render_footer": "plugin supplied",
		},
	}); err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatal("expected rendered document")
	}
	if !strings.Contains(doc.HTML, "Created By RayleaBot 开发版本 &amp; Plugin 系统模板 开发版本") {
		t.Fatalf("footer was not injected with system context: %s", doc.HTML)
	}
	if strings.Contains(doc.HTML, "plugin supplied") {
		t.Fatalf("plugin-supplied footer should be overwritten: %s", doc.HTML)
	}
}

func TestRenderInjectsFooterWithPluginContextAndBuildVersion(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")
	if err := os.WriteFile(filepath.Join(repoRoot, "build_info.json"), []byte(`{"version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("write build_info.json: %v", err)
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
		FooterTemplate:     "Core {{rayleabot_version}} / {{plugin_name}} {{plugin_version}} / {{unknown}}",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := service.Render(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "帮助",
		},
		Plugin: &PluginContext{
			Name:    "运势",
			Version: "0.4.0",
		},
	}); err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatal("expected rendered document")
	}
	if !strings.Contains(doc.HTML, "Core 1.2.3 / 运势 0.4.0 / {{unknown}}") {
		t.Fatalf("footer was not injected with plugin context: %s", doc.HTML)
	}
}

func TestServicePreviewHTMLReusesValidationAndSkipsRunnerAndArtifacts(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

	runner := &fakeRunner{}
	outputRoot := filepath.Join(t.TempDir(), "render-output")
	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
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

	preview, err := service.PreviewHTML(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title":         "帮助菜单",
			"render_footer": "plugin supplied",
		},
	})
	if err != nil {
		t.Fatalf("PreviewHTML: %v", err)
	}
	if preview.TemplateID != "help.menu" || preview.RevisionID == "" {
		t.Fatalf("unexpected preview identity: %#v", preview)
	}
	if preview.Width != 960 || preview.Height != 640 {
		t.Fatalf("preview dimensions = %dx%d, want 960x640", preview.Width, preview.Height)
	}
	if !strings.Contains(preview.HTML, "帮助菜单") || !strings.Contains(preview.HTML, "Created By RayleaBot 开发版本 &amp; Plugin 系统模板 开发版本") {
		t.Fatalf("preview html missing rendered data or footer: %s", preview.HTML)
	}
	if strings.Contains(preview.HTML, "plugin supplied") {
		t.Fatalf("plugin-supplied footer should be overwritten: %s", preview.HTML)
	}
	if runner.callCount() != 0 {
		t.Fatalf("runner call count = %d, want 0", runner.callCount())
	}
	entries, err := os.ReadDir(outputRoot)
	if err != nil {
		t.Fatalf("read output root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("preview html wrote render output files: %#v", entries)
	}

	_, err = service.PreviewHTML(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": make(chan int),
		},
	})
	var renderErr *Error
	if !errors.As(err, &renderErr) || renderErr.Code != "platform.invalid_request" {
		t.Fatalf("expected invalid request for unserializable input, got %v", err)
	}
}

func TestServicePreviewHTMLCachesByRevisionThemeAndData(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

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

	first, err := service.PreviewHTML(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "第一次",
		},
	})
	if err != nil {
		t.Fatalf("PreviewHTML first: %v", err)
	}
	second, err := service.PreviewHTML(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "第一次",
		},
	})
	if err != nil {
		t.Fatalf("PreviewHTML second: %v", err)
	}
	if second != first {
		t.Fatalf("same revision and data should reuse cached preview\nfirst=%#v\nsecond=%#v", first, second)
	}

	changedData, err := service.PreviewHTML(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "第二次",
		},
	})
	if err != nil {
		t.Fatalf("PreviewHTML changed data: %v", err)
	}
	if changedData.HTML == first.HTML {
		t.Fatalf("changed data should render different html")
	}

	templatePath := filepath.Join(templatesRoot, "help.menu", "template.html")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	content = []byte(strings.Replace(string(content), "</body>", "<p>revision marker</p></body>", 1))
	if err := os.WriteFile(templatePath, content, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	revised, err := service.PreviewHTML(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "第一次",
		},
	})
	if err != nil {
		t.Fatalf("PreviewHTML revised: %v", err)
	}
	if revised.RevisionID == first.RevisionID {
		t.Fatalf("template file change should create a new revision")
	}
	if !strings.Contains(revised.HTML, "revision marker") {
		t.Fatalf("revised html should reflect template changes: %s", revised.HTML)
	}
}

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

func TestServiceRenderCachesArtifacts(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-cache")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
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

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
			"items": []map[string]any{
				{"name": "weather", "description": "查询天气", "usage": "/weather <城市>"},
			},
		},
	}

	first, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("first Render: %v", err)
	}
	if first.FromCache {
		t.Fatalf("expected first render to miss cache")
	}
	if first.ArtifactID == "" || first.CacheKey == "" || first.ImagePath == "" {
		t.Fatalf("expected artifact metadata, got %#v", first)
	}

	second, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if !second.FromCache {
		t.Fatalf("expected second render to hit cache")
	}
	if second.ArtifactID != first.ArtifactID || second.CacheKey != first.CacheKey {
		t.Fatalf("expected stable cache result: first=%#v second=%#v", first, second)
	}
	if runner.callCount() != 1 {
		t.Fatalf("runner call count = %d, want 1", runner.callCount())
	}
}

func TestServiceRenderUsesConfiguredDefaultsAndExplicitOutput(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(repoRoot, "templates"), "help.menu")
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
		DefaultOutput:      "jpeg",
		DeviceScalePercent: 200,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	defaulted, err := service.Render(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	})
	if err != nil {
		t.Fatalf("Render defaulted output: %v", err)
	}
	if defaulted.MIME != "image/jpeg" || !strings.HasSuffix(defaulted.ImagePath, ".jpg") {
		t.Fatalf("defaulted output result = %#v, want jpeg artifact", defaulted)
	}
	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatal("expected render document")
	}
	if doc.Output != "jpeg" {
		t.Fatalf("document output = %q, want jpeg", doc.Output)
	}
	if doc.DeviceScaleFactor != 2 {
		t.Fatalf("device scale factor = %v, want 2", doc.DeviceScaleFactor)
	}

	explicit, err := service.Render(context.Background(), Request{
		Template: "help.menu",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	})
	if err != nil {
		t.Fatalf("Render explicit output: %v", err)
	}
	if explicit.MIME != "image/png" || !strings.HasSuffix(explicit.ImagePath, ".png") {
		t.Fatalf("explicit output result = %#v, want png artifact", explicit)
	}
}

func TestServiceRenderCacheKeyTracksDeviceScalePercent(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(repoRoot, "templates"), "help.menu")
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
		DeviceScalePercent: 100,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	request := Request{
		Template: "help.menu",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	}
	first, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("first Render: %v", err)
	}
	service.UpdateRuntimeConfig(RuntimeConfig{DeviceScalePercent: 200})
	second, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if second.FromCache {
		t.Fatal("expected scale change to miss previous cache")
	}
	if second.CacheKey == first.CacheKey || second.ArtifactID == first.ArtifactID {
		t.Fatalf("scale change reused cache: first=%#v second=%#v", first, second)
	}
	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatal("expected render document")
	}
	if doc.DeviceScaleFactor != 2 {
		t.Fatalf("device scale factor = %v, want 2", doc.DeviceScaleFactor)
	}
	if runner.callCount() != 2 {
		t.Fatalf("runner call count = %d, want 2", runner.callCount())
	}
}

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
	if err := service.SyncPluginTemplates(context.Background(), []PluginTemplateSource{{
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
		t.Fatalf("plugin template not listed with source: %#v", items)
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
	if err := os.WriteFile(filepath.Join(root, "outside.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write outside html: %v", err)
	}
	files := map[string]string{
		"template.json": `{
  "id": "card",
  "version": "1",
  "entry_html": "../outside.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 320,
  "height": 240
}`,
		"styles.css":        "body { margin: 0; }",
		"input.schema.json": `{"type":"object"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template file %s: %v", name, err)
		}
	}

	err := ValidatePluginTemplateSources([]PluginTemplateSource{{
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

	err := ValidatePluginTemplateSources([]PluginTemplateSource{{
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

	err = service.SyncPluginTemplates(context.Background(), []PluginTemplateSource{{
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
	if err := service.SyncPluginTemplates(context.Background(), []PluginTemplateSource{{
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
	if err := service.SyncPluginTemplates(context.Background(), []PluginTemplateSource{{
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
	var renderErr *Error
	if !errors.As(err, &renderErr) || renderErr.Code != "permission.scope_violation" {
		t.Fatalf("expected permission.scope_violation, got %v", err)
	}
}

func TestServiceRenderRequestsAdaptiveDocumentHeight(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

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

	_, err = service.Render(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "帮助菜单",
			"items": []map[string]any{
				{"name": "weather", "description": "查询天气"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatalf("expected render document")
	}
	if !doc.AutoHeight {
		t.Fatalf("expected render document to request adaptive height")
	}
	if doc.Width != 960 || doc.Height != 640 {
		t.Fatalf("unexpected initial render dimensions: got %dx%d", doc.Width, doc.Height)
	}
	if doc.BaseURL == "" || !strings.HasPrefix(doc.BaseURL, "file:") || !strings.HasSuffix(doc.BaseURL, "/templates/help.menu/") {
		t.Fatalf("unexpected template base URL: %q", doc.BaseURL)
	}
}

func TestServiceRenderLeaderboardListTemplate(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-leaderboard")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
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

	_, err = service.Render(context.Background(), Request{
		Template: "leaderboard.list",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title":       "本周发言榜",
			"subtitle":    "统计周期：2026-05-01 至 2026-05-07",
			"value_label": "发言数",
			"items": []map[string]any{
				{
					"avatar_url":     "https://q.qlogo.cn/headimg_dl?dst_uin=10001&spec=640",
					"group_nickname": "测试群名片",
					"nickname":       "Silver",
					"title":          "群主",
					"value":          128,
				},
				{
					"nickname": "Nova",
					"value":    81,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatalf("expected render document")
	}
	if !doc.AutoHeight {
		t.Fatalf("expected render document to request adaptive height")
	}
	if doc.Width != 960 || doc.Height != 420 {
		t.Fatalf("unexpected initial render dimensions: got %dx%d", doc.Width, doc.Height)
	}
	for _, want := range []string{"测试群名片", "（Silver）", "群主", "Nova", "128", "81"} {
		if !strings.Contains(doc.HTML, want) {
			t.Fatalf("leaderboard html missing %q:\n%s", want, doc.HTML)
		}
	}
}

func TestServiceRenderFortuneCardTemplate(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-fortune")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
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

	_, err = service.Render(context.Background(), Request{
		Template: "fortune.card",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title":         "今日运势",
			"subtitle":      "2026-05-04",
			"repeat_notice": "今日运势已经抽取过，以下为当日结果。",
			"user": map[string]any{
				"group_nickname": "测试群名片",
				"nickname":       "Silver",
				"title":          "群主",
				"id":             "10001",
			},
			"group": map[string]any{
				"name": "测试群",
			},
			"fortune": map[string]any{
				"name":        "大吉",
				"stars":       "★★★★★★★",
				"sign":        "云开见月，万事顺遂",
				"explanation": "适合推进重要事项。",
			},
			"today_good": []string{"整理计划", "主动沟通"},
			"today_bad":  []string{"熬夜", "拖延决定"},
			"streak": map[string]any{
				"current": 7,
				"total":   12,
			},
			"stats": []map[string]any{
				{"label": "累计大吉", "value": "2 次"},
				{"label": "最长连续大凶", "value": "1 天"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatalf("expected render document")
	}
	if !doc.AutoHeight {
		t.Fatalf("expected render document to request adaptive height")
	}
	if doc.Width != 1124 || doc.Height != 1365 {
		t.Fatalf("unexpected initial render dimensions: got %dx%d", doc.Width, doc.Height)
	}
	if doc.BaseURL == "" || !strings.HasPrefix(doc.BaseURL, "file:") || !strings.HasSuffix(doc.BaseURL, "/templates/fortune.card/") {
		t.Fatalf("unexpected template base URL: %q", doc.BaseURL)
	}
	for _, want := range []string{"今日运势", "今日运势已经抽取过", "测试群名片", "群主", "大吉", "★★★★★★★", "连续签到"} {
		if !strings.Contains(doc.HTML, want) {
			t.Fatalf("fortune html missing %q:\n%s", want, doc.HTML)
		}
	}
	for _, unwanted := range []string{"累计大吉", "最长连续大凶", "运势统计"} {
		if strings.Contains(doc.HTML, unwanted) {
			t.Fatalf("fortune html contains %q:\n%s", unwanted, doc.HTML)
		}
	}
}

func TestServiceRenderRejectsInputTooLarge(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-limit")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     1,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 32,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	_, err = service.Render(context.Background(), Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": strings.Repeat("x", 128),
		},
	})
	if err == nil {
		t.Fatal("expected oversized render data error")
	}

	var renderErr *Error
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if renderErr.Code != "platform.render_input_too_large" {
		t.Fatalf("unexpected error code: got %q want %q", renderErr.Code, "platform.render_input_too_large")
	}
}

func TestChromiumRunnerLoadsRelativeTemplateAssets(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	browserPath, err := deps.NewManager(repoRoot).ResolvePreparedEntrypoint("chromium", "browser")
	if err != nil {
		t.Skipf("managed chromium is not prepared: %v", err)
	}

	templatesRoot := filepath.Join(t.TempDir(), "templates")
	assetDir := filepath.Join(templatesRoot, "asset.check", "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("create asset dir: %v", err)
	}
	asset, err := os.Create(filepath.Join(assetDir, "red.png"))
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := png.Encode(asset, singlePixel(color.RGBA{R: 240, G: 16, B: 16, A: 255})); err != nil {
		_ = asset.Close()
		t.Fatalf("encode asset: %v", err)
	}
	if err := asset.Close(); err != nil {
		t.Fatalf("close asset: %v", err)
	}

	runner := NewChromiumRunner(ChromiumOptions{BrowserPath: browserPath})
	content, err := runner.Render(context.Background(), Document{
		Template:   "relative.asset",
		Output:     "png",
		BaseURL:    templateBaseURL(filepath.Join(templatesRoot, "asset.check")),
		Width:      320,
		Height:     240,
		AutoHeight: true,
		HTML: `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <style>
      :root {
        --asset-smoke: url("assets/red.png");
      }
      body { margin: 0; }
      main {
        width: 320px;
        height: 240px;
        background: #ffffff var(--asset-smoke) center / cover no-repeat;
      }
    </style>
  </head>
  <body><main aria-label="relative asset smoke"></main></body>
</html>`,
	})
	if err != nil {
		t.Fatalf("Render with relative asset: %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("expected screenshot content")
	}

	screenshot, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	r, g, b, _ := screenshot.At(160, 120).RGBA()
	if r>>8 < 220 || g>>8 > 40 || b>>8 > 40 {
		t.Fatalf("relative asset did not paint expected pixel: got rgb(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

func singlePixel(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	return img
}

func TestServiceRenderRejectsQueueFull(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-queue")
	waitCh := make(chan struct{})
	runner := &fakeRunner{waitCh: waitCh}
	var closeWait sync.Once
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     1,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      2 * time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		closeWait.Do(func() {
			close(waitCh)
		})
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Render(context.Background(), request)
		firstDone <- err
	}()

	secondDone := make(chan error, 1)
	go func() {
		_, err := service.Render(context.Background(), request)
		secondDone <- err
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runner.callCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	_, err = service.Render(context.Background(), request)
	if err == nil {
		t.Fatal("expected queue full error")
	}

	var renderErr *Error
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if renderErr.Code != "platform.render_queue_full" {
		t.Fatalf("unexpected error code: got %q want %q", renderErr.Code, "platform.render_queue_full")
	}

	closeWait.Do(func() {
		close(waitCh)
	})
	if err := <-firstDone; err != nil {
		t.Fatalf("first render failed after release: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second render failed after release: %v", err)
	}
}

func TestServiceSyncsTemplateFileChangesAfterRestart(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

	baseDir := t.TempDir()
	dbPath := filepath.Join(baseDir, "render-state.db")
	outputRoot := filepath.Join(baseDir, "render-output")

	service, cleanup := openPersistentRenderService(t, repoRoot, dbPath, outputRoot, &fakeRunner{})
	list, err := service.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(list) != 1 || list[0].ID != "help.menu" {
		t.Fatalf("expected seeded templates, got %#v", list)
	}

	baseRevisionID, source, err := service.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource: %v", err)
	}
	source.HTML = `<section class="persisted">{{ .title }}</section>`

	detail, err := service.UpdateTemplateSource(context.Background(), "help.menu", baseRevisionID, "调整模板内容", source)
	if err != nil {
		t.Fatalf("UpdateTemplateSource: %v", err)
	}
	persistedRevisionID := detail.CurrentRevision.RevisionID
	cleanup()

	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "template.html"), []byte(`<section class="file">{{ .title }}</section>`), 0o644); err != nil {
		t.Fatalf("write updated template html: %v", err)
	}

	reopened, cleanupReopened := openPersistentRenderService(t, repoRoot, dbPath, outputRoot, &fakeRunner{})
	defer cleanupReopened()

	reopenedDetail, err := reopened.GetTemplate(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplate after restart: %v", err)
	}
	if reopenedDetail.CurrentRevision.RevisionID == persistedRevisionID {
		t.Fatalf("current revision did not track updated template file")
	}

	_, reopenedSource, err := reopened.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource after restart: %v", err)
	}
	if reopenedSource.HTML != `<section class="file">{{ .title }}</section>` {
		t.Fatalf("template source did not track file update: %q", reopenedSource.HTML)
	}
}

func TestServiceRenderCacheKeyTracksStoredSourceDigest(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

	outputRoot := filepath.Join(t.TempDir(), "render-output")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
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

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	}

	first, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("Render current template: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "styles.css"), []byte("body { margin: 0; }\n.fresh { color: red; }"), 0o644); err != nil {
		t.Fatalf("write updated template stylesheet: %v", err)
	}

	second, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("Render synced template: %v", err)
	}
	if second.FromCache {
		t.Fatalf("expected synced template render to miss previous cache")
	}
	if second.ArtifactID == first.ArtifactID {
		t.Fatalf("synced template reused stale artifact id")
	}

	third, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("Render cached synced template: %v", err)
	}
	if !third.FromCache {
		t.Fatalf("expected synced template to hit cache on repeated render")
	}
	if runner.callCount() != 2 {
		t.Fatalf("unexpected runner calls: got %d want 2", runner.callCount())
	}
}

func TestServiceRenderCacheKeyTracksTemplateAssets(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")
	assetDir := filepath.Join(templatesRoot, "help.menu", "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("create asset dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "badge.png"), []byte("first"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
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

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	}

	first, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("Render with initial asset: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "badge.png"), []byte("second"), 0o644); err != nil {
		t.Fatalf("write updated asset: %v", err)
	}

	second, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("Render with updated asset: %v", err)
	}
	if second.FromCache {
		t.Fatalf("expected asset change render to miss previous cache")
	}
	if second.ArtifactID == first.ArtifactID {
		t.Fatalf("asset change reused stale artifact id")
	}
	if runner.callCount() != 2 {
		t.Fatalf("unexpected runner calls: got %d want 2", runner.callCount())
	}
}

func TestServiceTemplateReadsSyncChangedFiles(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

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

	before, err := service.GetTemplate(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplate before update: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "styles.css"), []byte("body { margin: 0; }\n.synced { color: red; }"), 0o644); err != nil {
		t.Fatalf("write updated template stylesheet: %v", err)
	}

	list, err := service.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates after update: %v", err)
	}
	if len(list) != 1 || list[0].ID != "help.menu" {
		t.Fatalf("unexpected templates: %#v", list)
	}
	if list[0].UpdatedAt == before.UpdatedAt {
		t.Fatalf("template updated_at did not change after file sync")
	}

	_, source, err := service.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource after update: %v", err)
	}
	if !strings.Contains(source.Stylesheet, ".synced") {
		t.Fatalf("template source did not include updated stylesheet: %q", source.Stylesheet)
	}
}

func TestServiceInvalidTemplateFileKeepsCurrentRevision(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

	var logs bytes.Buffer
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
		Logger:             slog.New(slog.NewTextHandler(&logs, nil)),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	before, source, err := service.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource before invalid file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "template.html"), []byte("{{ if }}"), 0o644); err != nil {
		t.Fatalf("write invalid template html: %v", err)
	}

	after, afterSource, err := service.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource after invalid file: %v", err)
	}
	if after != before {
		t.Fatalf("invalid template file changed current revision: got %q want %q", after, before)
	}
	if afterSource.HTML != source.HTML {
		t.Fatalf("invalid template file replaced current source")
	}
	if !strings.Contains(logs.String(), "render template skipped") {
		t.Fatalf("expected invalid template warning, got %q", logs.String())
	}
}

func TestServiceValidateTemplateRejectsInvalidManifestAndReportsCompileIssues(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-output")
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
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

	_, source, err := service.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource: %v", err)
	}

	invalidManifest := source
	invalidManifest.ManifestJSON = map[string]any{"version": "1"}

	_, err = service.ValidateTemplate(context.Background(), "help.menu", &invalidManifest)
	if err == nil {
		t.Fatal("expected invalid manifest error")
	}

	var renderErr *Error
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if renderErr.Code != "platform.template_source_invalid" {
		t.Fatalf("unexpected error code: got %q want %q", renderErr.Code, "platform.template_source_invalid")
	}

	invalidHTML := source
	invalidHTML.HTML = "{{ if }}"

	result, err := service.ValidateTemplate(context.Background(), "help.menu", &invalidHTML)
	if err != nil {
		t.Fatalf("ValidateTemplate invalid html: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid html validation to fail")
	}
	if len(result.Issues) != 1 || result.Issues[0].Code != "html.compile_failed" {
		t.Fatalf("unexpected validation issues: %#v", result.Issues)
	}

	detail, err := service.templateRepo.GetTemplateDetail(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateDetail: %v", err)
	}
	if detail.LastValidation.Valid {
		t.Fatalf("expected last validation status to reflect failed compile")
	}
	if detail.LastValidation.IssueCount != 1 {
		t.Fatalf("unexpected validation issue count: got %d want 1", detail.LastValidation.IssueCount)
	}

	syncedDetail, err := service.GetTemplate(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplate after sync: %v", err)
	}
	if !syncedDetail.LastValidation.Valid {
		t.Fatalf("expected valid file sync to restore validation status")
	}
}

func TestManifestPlatformNormalizesWindowsAMD64(t *testing.T) {
	t.Parallel()

	if got := deps.ManifestPlatform("windows", "amd64"); got != "windows-x64" {
		t.Fatalf("manifestPlatform(windows, amd64) = %q, want windows-x64", got)
	}
	if got := deps.ManifestPlatform("darwin", "arm64"); got != "macos-arm64" {
		t.Fatalf("manifestPlatform(darwin, arm64) = %q, want macos-arm64", got)
	}
}

func openRenderTestStore(t *testing.T) *storage.Store {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "render-state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close store: %v", err)
		}
	})
	return store
}

func writeRenderTemplateSeed(t *testing.T, templatesRoot, templateID string) {
	t.Helper()

	templateDir := filepath.Join(templatesRoot, templateID)
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}

	manifest := fmt.Sprintf(`{
  "id": %q,
  "version": "1",
  "entry_html": "template.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 960,
  "height": 640
}`, templateID)
	files := map[string]string{
		"template.json":     manifest,
		"template.html":     "<html><body>{{ .title }} {{ .render_footer }}</body></html>",
		"styles.css":        "body { margin: 0; }",
		"input.schema.json": `{"type":"object"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template file %s: %v", name, err)
		}
	}
}

func openPersistentRenderService(t *testing.T, repoRoot, dbPath, outputRoot string, runner Runner) (*Service, func()) {
	t.Helper()

	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		_ = store.Close()
		t.Fatalf("NewService: %v", err)
	}

	return service, func() {
		_ = service.Close()
		_ = store.Close()
	}
}

// recordingRenderMetrics captures every render outcome and queue depth
// signal so TestServiceRenderRecordsMetrics can assert the observer hooks
// used by /api/system/metrics actually fire.
type recordingRenderMetrics struct {
	mu              sync.Mutex
	durations       []renderMetricSample
	maxQueueDepth   int
	queueDepthCalls int
}

type renderMetricSample struct {
	outcome  string
	duration time.Duration
}

func (m *recordingRenderMetrics) SetRenderQueueDepth(depth int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueDepthCalls++
	if depth > m.maxQueueDepth {
		m.maxQueueDepth = depth
	}
}

func (m *recordingRenderMetrics) ObserveRenderDuration(outcome string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations = append(m.durations, renderMetricSample{outcome: outcome, duration: duration})
}

// TestServiceRenderRecordsMetrics verifies the render service drives the
// configured MetricsObserver for both successful renders and cache hits.
// The /api/system/metrics contract advertises render_queue_depth and
// render_duration_seconds; this test guards the actual write paths.
func TestServiceRenderRecordsMetrics(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-metrics")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
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

	metrics := &recordingRenderMetrics{}
	service.SetMetricsObserver(metrics)

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
			"items": []map[string]any{
				{"name": "weather", "description": "查询天气", "usage": "/weather <城市>"},
			},
		},
	}

	if _, err := service.Render(context.Background(), request); err != nil {
		t.Fatalf("first Render: %v", err)
	}
	if _, err := service.Render(context.Background(), request); err != nil {
		t.Fatalf("second Render: %v", err)
	}

	// SetRenderQueueDepth runs in a goroutine; give it a moment.
	time.Sleep(50 * time.Millisecond)

	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if len(metrics.durations) != 2 {
		t.Fatalf("durations = %d, want 2", len(metrics.durations))
	}
	outcomes := map[string]int{}
	for _, sample := range metrics.durations {
		outcomes[sample.outcome]++
	}
	if outcomes["succeeded"] != 1 {
		t.Fatalf("succeeded count = %d, want 1", outcomes["succeeded"])
	}
	if outcomes["cache_hit"] != 1 {
		t.Fatalf("cache_hit count = %d, want 1", outcomes["cache_hit"])
	}
	if metrics.queueDepthCalls == 0 {
		t.Fatal("expected at least one queue-depth update")
	}
	if metrics.maxQueueDepth < 1 {
		t.Fatalf("maxQueueDepth = %d, want >= 1", metrics.maxQueueDepth)
	}
}
