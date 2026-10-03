package render

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestServiceCompilationTracksSameFingerprintChanges(t *testing.T) {
	for _, changedFile := range []string{"html", "stylesheet", "schema"} {
		t.Run(changedFile, func(t *testing.T) {
			root := t.TempDir()
			writeRenderTemplateSeed(t, filepath.Join(root, "templates"), "card")
			dir := filepath.Join(root, "templates", "card")
			files := map[string]string{
				"template.HTML":     `<html><head><style>{{ .stylesheet }}</style></head><body>before {{ .title }}</body></html>`,
				"styles.css":        "body { color: red; }",
				"input.Schema.json": `{"type":"object","properties":{"title":{"type":"string"}}}`,
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			runner := &fakeRunner{}
			service, _ := newCompilationTestService(t, root, runner)
			request := Request{Template: "card", Output: "png", Data: map[string]any{"title": "content"}}
			first, err := service.Render(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.PreviewHTML(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			switch changedFile {
			case "html":
				replaceSameFingerprint(t, filepath.Join(dir, "template.HTML"), "before", "after!")
			case "stylesheet":
				replaceSameFingerprint(t, filepath.Join(dir, "styles.css"), "red", "tan")
			case "schema":
				replaceSameFingerprint(t, filepath.Join(dir, "input.Schema.json"), "string", "number")
			}
			second, err := service.Render(context.Background(), request)
			if changedFile == "schema" {
				if info, ok := AsTemplateError(err); !ok || info.Code != "platform.invalid_request" {
					t.Fatalf("updated schema accepted string input: %v", err)
				}
				if _, err := service.PreviewHTML(context.Background(), request); err == nil {
					t.Fatal("preview used the previous schema")
				}
				if runner.callCount() != 1 {
					t.Fatal("invalid input reached the renderer")
				}
				return
			}
			if err != nil || second.FromCache || first.ArtifactID == second.ArtifactID {
				t.Fatalf("changed source render = %+v, error = %v", second, err)
			}
			preview, err := service.PreviewHTML(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			want := "after!"
			if changedFile == "stylesheet" {
				want = "color: tan"
			}
			document, ok := runner.lastDocument()
			if !ok || !strings.Contains(document.HTML, want) || !strings.Contains(preview.HTML, want) {
				t.Fatalf("changed content not used: document=%q preview=%q", document.HTML, preview.HTML)
			}
		})
	}
}

func TestServiceCompilationReadsCurrentDatabaseContent(t *testing.T) {
	root := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(root, "templates"), "card")
	service, store := newCompilationTestService(t, root, &fakeRunner{})
	request := Request{Template: "card", Data: map[string]any{"title": "original"}}
	first, err := service.PreviewHTML(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	// A stored digest cannot establish that the rest of the row is unchanged.
	if _, err := store.Write.Exec(`UPDATE render_templates SET html = ? WHERE template_id = ?`, `<html><body>database content {{ .title }}</body></html>`, "card"); err != nil {
		t.Fatal(err)
	}
	second, err := service.PreviewHTML(context.Background(), Request{Template: "card", Data: map[string]any{"title": "fresh input"}})
	if err != nil || !strings.Contains(second.HTML, "database content fresh input") {
		t.Fatalf("database content was bypassed: %+v, %v", second, err)
	}
	if second.SourceDigest != first.SourceDigest {
		t.Fatal("source digest no longer preserves the stored digest behavior")
	}
	if _, err := store.Write.Exec(`UPDATE render_templates SET source_type = 'plugin', source_plugin_id = 'owner', source_local_id = 'card' WHERE template_id = 'card'`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Render(context.Background(), request); err == nil {
		t.Fatal("cached compilation bypassed the source ownership conflict")
	}
}

func TestServiceCompilationRejectsUnavailableSourcesAndReleasesEntries(t *testing.T) {
	for _, change := range []string{"invalid", "removed"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			writeRenderTemplateSeed(t, filepath.Join(root, "templates"), "card")
			service, _ := newCompilationTestService(t, root, &fakeRunner{})
			request := Request{Template: "card", Data: map[string]any{"title": "cached"}}
			if _, err := service.Render(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "templates", "card", "template.HTML")
			if change == "invalid" {
				if err := os.WriteFile(path, []byte("{{ if }}"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Render(context.Background(), request); err == nil {
				t.Fatal("unavailable template rendered from cache")
			}
			if _, err := service.PreviewHTML(context.Background(), request); err == nil {
				t.Fatal("unavailable template previewed from cache")
			}
			service.templateCompiler.mu.Lock()
			_, retained := service.templateCompiler.entries["card"]
			service.templateCompiler.mu.Unlock()
			if retained {
				t.Fatal("unavailable template retains its compiled source")
			}
			if err := os.WriteFile(path, []byte(`<html><body>restored {{ .title }}</body></html>`), 0o644); err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewHTML(context.Background(), request)
			if err != nil || !strings.Contains(preview.HTML, "restored cached") {
				t.Fatalf("restored source was not compiled: %+v, %v", preview, err)
			}
		})
	}
}

func TestServiceCompilationSupportsConcurrentIndependentRequests(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"first", "second"} {
		writeRenderTemplateSeed(t, filepath.Join(root, "templates"), id)
	}
	service, _ := newCompilationTestService(t, root, &fakeRunner{})
	start := make(chan struct{})
	errors := make(chan error, 8)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			<-start
			for round := 0; round < 4; round++ {
				id := []string{"first", "second"}[(worker+round)%2]
				title := fmt.Sprintf("worker-%d-round-%d", worker, round)
				request := Request{Template: id, Data: map[string]any{"title": title}}
				preview, err := service.PreviewHTML(context.Background(), request)
				if err != nil || !strings.Contains(preview.HTML, title) {
					errors <- fmt.Errorf("preview %s: %v, html=%q", title, err, preview.HTML)
					return
				}
				if _, err := service.Render(context.Background(), request); err != nil {
					errors <- err
					return
				}
				_, source, err := service.GetTemplateSource(context.Background(), id)
				if err != nil {
					errors <- err
					return
				}
				source.ManifestJSON["name"] = "caller changes"
				source.InputSchemaJSON["type"] = "string"
			}
		}(worker)
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	for _, id := range []string{"first", "second"} {
		preview, err := service.PreviewHTML(context.Background(), Request{Template: id, Data: map[string]any{"title": "after readers"}})
		if err != nil || !strings.Contains(preview.HTML, "after readers") {
			t.Fatalf("catalog caller changed service compilation: %+v, %v", preview, err)
		}
	}
}

func newCompilationTestService(t *testing.T, root string, runner Runner) (*Service, *storage.Store) {
	t.Helper()
	store := openRenderTestStore(t)
	service, err := NewService(Options{
		RepoRoot: root, OutputRoot: filepath.Join(root, "output"), Store: store,
		Runner: runner, BrowserPath: "unused-test-browser", WorkerCount: 4,
		QueueMaxLength: 32, QueueWaitTimeout: 5 * time.Second, RenderTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	})
	return service, store
}

func replaceSameFingerprint(t *testing.T, path, old, next string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := bytes.Replace(content, []byte(old), []byte(next), 1)
	if len(updated) != len(content) || bytes.Equal(updated, content) {
		t.Fatal("source replacement must change content while preserving size")
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}
