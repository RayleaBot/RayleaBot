package service

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

	detail, err := service.GetTemplate(context.Background(), "help.menu")
	if err != nil {
		t.Fatal(err)
	}
	persistedSourceDigest := detail.SourceDigest
	cleanup()

	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "template.HTML"), []byte(`<section class="file">{{ .title }}</section>`), 0o644); err != nil {
		t.Fatalf("write updated template HTML: %v", err)
	}

	reopened, cleanupReopened := openPersistentRenderService(t, repoRoot, dbPath, outputRoot, &fakeRunner{})
	defer cleanupReopened()

	reopenedDetail, err := reopened.GetTemplate(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplate after restart: %v", err)
	}
	if reopenedDetail.SourceDigest == persistedSourceDigest {
		t.Fatalf("current source digest did not track updated template file")
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

	stylesheetPath := filepath.Join(templatesRoot, "help.menu", "styles.css")
	if err := os.WriteFile(stylesheetPath, []byte("body { margin: 0; }\n.fresh { color: red; }"), 0o644); err != nil {
		t.Fatalf("write updated template Stylesheet: %v", err)
	}
	updatedModTime := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(stylesheetPath, updatedModTime, updatedModTime); err != nil {
		t.Fatalf("set updated template Stylesheet mtime: %v", err)
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

	stylesheetPath := filepath.Join(templatesRoot, "help.menu", "styles.css")
	if err := os.WriteFile(stylesheetPath, []byte("body { margin: 0; }\n.synced { color: red; }"), 0o644); err != nil {
		t.Fatalf("write updated template Stylesheet: %v", err)
	}
	updatedModTime := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(stylesheetPath, updatedModTime, updatedModTime); err != nil {
		t.Fatalf("set updated template Stylesheet mtime: %v", err)
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
		t.Fatalf("template source did not include updated Stylesheet: %q", source.Stylesheet)
	}
}

func TestServiceInvalidTemplateFileInvalidatesCurrentCache(t *testing.T) {
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

	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "template.HTML"), []byte("{{ if }}"), 0o644); err != nil {
		t.Fatalf("write invalid template HTML: %v", err)
	}

	if _, _, err := service.GetTemplateSource(context.Background(), "help.menu"); err == nil {
		t.Fatal("invalid source must not fall back to stored content")
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "template_dir=templates/help.menu") {
		t.Fatalf("expected invalid template warning, got %q", logs.String())
	}
}
