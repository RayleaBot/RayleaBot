package render

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
