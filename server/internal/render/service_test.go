package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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
		"template.html":     "<html><body>{{ .title }}</body></html>",
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
