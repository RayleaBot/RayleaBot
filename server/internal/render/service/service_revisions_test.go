package service

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

	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "template.HTML"), []byte(`<section class="file">{{ .title }}</section>`), 0o644); err != nil {
		t.Fatalf("write updated template HTML: %v", err)
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
		t.Fatalf("write updated template Stylesheet: %v", err)
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
		t.Fatalf("write updated template Stylesheet: %v", err)
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

	if err := os.WriteFile(filepath.Join(templatesRoot, "help.menu", "template.HTML"), []byte("{{ if }}"), 0o644); err != nil {
		t.Fatalf("write invalid template HTML: %v", err)
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

	repoRoot := filepath.Join("..", "..", "..", "..")
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
		t.Fatalf("ValidateTemplate invalid HTML: %v", err)
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
