package render

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCachedRenderReadsWithoutAcquiringOccupiedWriter(t *testing.T) {
	root := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(root, "templates"), "card")
	service, store := newCompilationTestService(t, root, &fakeRunner{})
	request := Request{Template: "card", Output: "png", Data: map[string]any{"title": "cached"}}
	first, err := service.Render(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	connection, err := store.Write.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	before := store.Write.Stats().WaitCount
	second, err := service.Render(ctx, request)
	if err != nil || !second.FromCache || second.ArtifactID != first.ArtifactID {
		t.Fatalf("unchanged render required writer access: %+v, %v", second, err)
	}
	if _, err := service.PreviewHTML(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetTemplateDetailSnapshot(ctx, "card"); err != nil {
		t.Fatal(err)
	}
	if got := store.Write.Stats().WaitCount; got != before {
		t.Fatalf("read operations waited for writer: %d → %d", before, got)
	}
}

func TestAssetLookupPreservesGlobalSourceProtectionAndInvalidManifestErrors(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"first", "second"} {
		writeRenderTemplateSeed(t, filepath.Join(root, "templates"), id)
	}
	assetPath := filepath.Join(root, "templates", "second", "assets", "badge.txt")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetPath, []byte("badge"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, store := newCompilationTestService(t, root, &fakeRunner{})
	asset, err := service.LookupTemplateAsset(t.Context(), "first", "../second/assets/badge.txt")
	if err != nil || asset.Path != assetPath {
		t.Fatalf("shared asset unavailable: %+v, %v", asset, err)
	}
	for _, source := range []string{"template.json", "template.HTML", "styles.css", "input.Schema.json", "preview.json"} {
		_, err := service.LookupTemplateAsset(t.Context(), "first", "../second/"+source)
		if info, ok := AsTemplateError(err); !ok || info.Code != "platform.resource_missing" {
			t.Fatalf("registered source %s escaped asset protection: %v", source, err)
		}
	}
	if _, err := store.Write.Exec(`UPDATE render_templates SET manifest_json = '{' WHERE template_id = 'second'`); err != nil {
		t.Fatal(err)
	}
	_, err = service.LookupTemplateAsset(t.Context(), "first", "../second/assets/badge.txt")
	var syntaxError *json.SyntaxError
	if !errors.As(err, &syntaxError) {
		t.Fatalf("global manifest decoding error was skipped: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.LookupTemplateAsset(ctx, "first", "../second/assets/badge.txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("asset cancellation lost: %v", err)
	}
}

func TestTemplateDetailSnapshotOwnsSourceAndRefreshesPreviewData(t *testing.T) {
	root := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(root, "templates"), "card")
	service, store := newCompilationTestService(t, root, &fakeRunner{})
	previewPath := filepath.Join(root, "templates", "card", DefaultPreviewData)
	if err := os.WriteFile(previewPath, []byte(`{"title":"first"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := service.GetTemplateDetailSnapshot(t.Context(), "card")
	if err != nil {
		t.Fatal(err)
	}
	first.Source.ManifestJSON["name"] = "caller mutation"
	first.Source.InputSchemaJSON["type"] = "string"
	first.PreviewData["title"] = "caller mutation"
	if err := os.WriteFile(previewPath, []byte(`{"title":"second"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write.Exec(`UPDATE render_templates SET html = '<html>current stored HTML</html>' WHERE template_id = 'card'`); err != nil {
		t.Fatal(err)
	}
	second, err := service.GetTemplateDetailSnapshot(t.Context(), "card")
	if err != nil || second.Source.ManifestJSON["name"] != second.Detail.Name || second.Source.ManifestJSON["name"] == "caller mutation" || second.Source.InputSchemaJSON["type"] != "object" || second.PreviewData["title"] != "second" || !strings.Contains(second.Source.HTML, "current stored HTML") {
		t.Fatalf("detail reused stale/shared fields: %+v, %v", second, err)
	}
}
