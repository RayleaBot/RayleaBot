package render

import (
	"context"
	"encoding/base64"
	"errors"
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
}

func (f *fakeRunner) Render(ctx context.Context, doc Document) ([]byte, error) {
	f.mu.Lock()
	f.calls++
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

func TestServiceSeedsTemplatesAndKeepsSavedRevisionOnRestart(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	baseDir := t.TempDir()
	dbPath := filepath.Join(baseDir, "render-state.db")
	outputRoot := filepath.Join(baseDir, "render-output")

	service, cleanup := openPersistentRenderService(t, repoRoot, dbPath, outputRoot, &fakeRunner{})
	list, err := service.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(list) < 2 {
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

	reopened, cleanupReopened := openPersistentRenderService(t, repoRoot, dbPath, outputRoot, &fakeRunner{})
	defer cleanupReopened()

	reopenedDetail, err := reopened.GetTemplate(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplate after restart: %v", err)
	}
	if reopenedDetail.CurrentRevision.RevisionID != persistedRevisionID {
		t.Fatalf("current revision changed after restart: got %q want %q", reopenedDetail.CurrentRevision.RevisionID, persistedRevisionID)
	}

	_, reopenedSource, err := reopened.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource after restart: %v", err)
	}
	if reopenedSource.HTML != source.HTML {
		t.Fatalf("persisted template source was overwritten on restart")
	}
}

func TestServiceRenderDraftDoesNotPersistAndCacheKeyTracksSourceDigest(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
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

	baseRevisionID, currentSource, err := service.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource: %v", err)
	}

	draftSource := currentSource
	draftSource.HTML = `<section class="draft">{{ .title }}</section>`
	draftResult, err := service.Render(context.Background(), Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
		Draft: &TemplateDraft{Source: draftSource},
	})
	if err != nil {
		t.Fatalf("Render draft template: %v", err)
	}
	if draftResult.ArtifactID == first.ArtifactID {
		t.Fatalf("draft render reused current artifact id")
	}

	revisionAfterDraft, sourceAfterDraft, err := service.GetTemplateSource(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplateSource after draft render: %v", err)
	}
	if revisionAfterDraft != baseRevisionID || sourceAfterDraft.HTML != currentSource.HTML {
		t.Fatalf("draft render changed current template revision")
	}

	currentSource.HTML = `<section class="saved">{{ .title }}</section>`
	if _, err := service.UpdateTemplateSource(context.Background(), "help.menu", baseRevisionID, "保存模板修改", currentSource); err != nil {
		t.Fatalf("UpdateTemplateSource: %v", err)
	}

	second, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("Render saved template: %v", err)
	}
	if second.FromCache {
		t.Fatalf("expected saved template render to miss previous cache")
	}
	if second.ArtifactID == first.ArtifactID {
		t.Fatalf("saved template reused stale artifact id")
	}

	third, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("Render cached saved template: %v", err)
	}
	if !third.FromCache {
		t.Fatalf("expected saved template to hit cache on repeated render")
	}
	if runner.callCount() != 3 {
		t.Fatalf("unexpected runner calls: got %d want 3", runner.callCount())
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

	detail, err := service.GetTemplate(context.Background(), "help.menu")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if detail.LastValidation.Valid {
		t.Fatalf("expected last validation status to reflect failed compile")
	}
	if detail.LastValidation.IssueCount != 1 {
		t.Fatalf("unexpected validation issue count: got %d want 1", detail.LastValidation.IssueCount)
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
