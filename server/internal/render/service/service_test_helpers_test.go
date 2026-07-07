package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

func singlePixel(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	return img
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
  "entry_html": "template.HTML",
  "stylesheet": "styles.css",
  "input_schema": "input.Schema.json",
  "width": 960,
  "height": 640
}`, templateID)
	files := map[string]string{
		"template.json":     manifest,
		"template.HTML":     "<html><body>{{ .title }} {{ .render_footer }}</body></html>",
		"styles.css":        "body { margin: 0; }",
		"input.Schema.json": `{"type":"object"}`,
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
