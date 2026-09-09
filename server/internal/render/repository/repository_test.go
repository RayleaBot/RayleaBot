package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestCurrentTemplateCacheReplacesContentAndPreservesOwnership(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repo, err := NewSQLiteTemplateRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	item := CurrentTemplate{ID: "card", SourceDigest: "one", UpdatedAt: "2026-09-09T00:00:00Z", Owner: TemplateSourceInfo{Type: "system"}, Source: TemplateSource{
		ManifestJSON: map[string]any{"id": "card", "name": "卡片", "version": "1", "entry_html": "template.html", "stylesheet": "styles.css", "width": 320, "height": 240}, HTML: "first",
	}}
	if changed, err := repo.SyncTemplate(ctx, item); err != nil || !changed {
		t.Fatalf("initial sync: %v %v", changed, err)
	}
	item.SourceDigest, item.Source.HTML = "two", "second"
	if changed, err := repo.SyncTemplate(ctx, item); err != nil || !changed {
		t.Fatalf("updated sync: %v %v", changed, err)
	}
	var count int
	if err := store.Read.QueryRow(`SELECT COUNT(*) FROM render_templates`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cache count=%d err=%v", count, err)
	}
	digest, source, err := repo.GetCurrentSource(ctx, "card")
	if err != nil || digest != "two" || source.HTML != "second" {
		t.Fatalf("current source: %q %+v %v", digest, source, err)
	}
	item.Owner = TemplateSourceInfo{Type: "plugin", PluginID: "other", LocalID: "card"}
	if _, err := repo.SyncTemplate(ctx, item); err == nil {
		t.Fatal("ownership collision accepted")
	}
	if err := repo.RemoveSystemTemplatesExcept(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Read.QueryRow(`SELECT COUNT(*) FROM render_templates`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("removed cache count=%d err=%v", count, err)
	}
}
