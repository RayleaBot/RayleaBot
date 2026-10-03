package render

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type templateSyncOutcome struct {
	changed bool
	err     error
}

func readTestTemplate() currentTemplate {
	return currentTemplate{ID: "card", SourceDigest: "initial", UpdatedAt: "2026-09-09T00:00:00Z", Owner: TemplateSourceInfo{Type: "system"}, Source: TemplateSource{
		ManifestJSON: map[string]any{"id": "card", "name": "Card", "version": "1", "entry_html": "template.html", "stylesheet": "styles.css", "width": 320, "height": 240}, HTML: "initial HTML",
	}}
}

func waitForTemplateWriter(t *testing.T, ctx context.Context, db *sql.DB, previous int64, done <-chan templateSyncOutcome) {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for db.Stats().WaitCount <= previous {
		select {
		case result := <-done:
			t.Fatalf("mutation completed without acquiring the occupied writer: %+v", result)
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("mutation did not reach the writer: %v", ctx.Err())
		}
	}
}

func TestTemplateSyncRechecksStateAfterWaitingForWriter(t *testing.T) {
	for _, scenario := range []string{"changed", "new owner", "matching digest", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			store := openRenderTestStore(t)
			repo, err := newTemplateRepository(store)
			if err != nil {
				t.Fatal(err)
			}
			item := readTestTemplate()
			if changed, err := repo.SyncTemplate(t.Context(), item); err != nil || !changed {
				t.Fatalf("initial sync: %v %v", changed, err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			connection, err := store.Write.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = connection.Close() }()
			previous := store.Write.Stats().WaitCount
			item.SourceDigest, item.Source.HTML = "changed", "changed HTML"
			done := make(chan templateSyncOutcome, 1)
			go func() {
				changed, err := repo.SyncTemplate(ctx, item)
				done <- templateSyncOutcome{changed: changed, err: err}
			}()
			waitForTemplateWriter(t, ctx, store.Write, previous, done)
			switch scenario {
			case "new owner":
				_, err = connection.ExecContext(ctx, `UPDATE render_templates SET source_type = 'plugin', source_plugin_id = 'other', source_local_id = 'card' WHERE template_id = 'card'`)
			case "matching digest":
				_, err = connection.ExecContext(ctx, `UPDATE render_templates SET source_digest = 'changed', html = 'concurrent HTML' WHERE template_id = 'card'`)
			case "cancelled":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := connection.Close(); err != nil {
				t.Fatal(err)
			}
			result := <-done
			digest, source, err := repo.GetCurrentSource(t.Context(), "card")
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "changed":
				if result.err != nil || !result.changed || digest != "changed" || source.HTML != "changed HTML" {
					t.Fatalf("changed row was not synchronized: %+v, %q, %q", result, digest, source.HTML)
				}
			case "new owner":
				if result.err == nil || errors.Is(result.err, context.DeadlineExceeded) || result.changed || digest != "initial" || source.HTML != "initial HTML" {
					t.Fatalf("ownership recheck failed: %+v, %q, %q", result, digest, source.HTML)
				}
				detail, err := repo.GetTemplateDetail(t.Context(), "card")
				if err != nil || detail.Source.Type != "plugin" || detail.Source.PluginID != "other" {
					t.Fatal("new owner was overwritten")
				}
			case "matching digest":
				if result.err != nil || result.changed || digest != "changed" || source.HTML != "concurrent HTML" {
					t.Fatalf("matching digest was not rechecked: %+v, %q, %q", result, digest, source.HTML)
				}
			case "cancelled":
				if !errors.Is(result.err, context.Canceled) || result.changed || digest != "initial" || source.HTML != "initial HTML" {
					t.Fatalf("cancelled sync changed the row: %+v, %q, %q", result, digest, source.HTML)
				}
			}
		})
	}
}

func TestTemplateRemovalWaitsAndRechecksItsPredicate(t *testing.T) {
	for _, changeOwner := range []bool{false, true} {
		name := "remove system row"
		if changeOwner {
			name = "preserve concurrent plugin owner"
		}
		t.Run(name, func(t *testing.T) {
			store := openRenderTestStore(t)
			repo, err := newTemplateRepository(store)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.SyncTemplate(t.Context(), readTestTemplate()); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			connection, err := store.Write.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = connection.Close() }()
			previous := store.Write.Stats().WaitCount
			done := make(chan templateSyncOutcome, 1)
			go func() { done <- templateSyncOutcome{err: repo.RemoveSystemTemplatesExcept(ctx, nil)} }()
			waitForTemplateWriter(t, ctx, store.Write, previous, done)
			if changeOwner {
				if _, err := connection.ExecContext(ctx, `UPDATE render_templates SET source_type = 'plugin', source_plugin_id = 'other', source_local_id = 'card' WHERE template_id = 'card'`); err != nil {
					t.Fatal(err)
				}
			}
			if err := connection.Close(); err != nil {
				t.Fatal(err)
			}
			if result := <-done; result.err != nil {
				t.Fatal(result.err)
			}
			detail, err := repo.GetTemplateDetail(t.Context(), "card")
			if changeOwner {
				if err != nil || detail.Source.Type != "plugin" || detail.Source.PluginID != "other" {
					t.Fatal("cleanup removed the new owner")
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("system row remains: %+v, %v", detail, err)
			}
		})
	}
}

func TestTemplateReadPreflightPreservesValidationAndCancellation(t *testing.T) {
	store := openRenderTestStore(t)
	repo, err := newTemplateRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	item := readTestTemplate()
	if _, err := repo.SyncTemplate(t.Context(), item); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := repo.SyncTemplate(ctx, item); !errors.Is(err, context.Canceled) {
		t.Fatalf("sync cancellation lost: %v", err)
	}
	if err := repo.RemoveSystemTemplatesExcept(ctx, []string{"card"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("empty cleanup ignored cancellation: %v", err)
	}
	invalid := item
	invalid.Source.ManifestJSON = map[string]any{"bad": func() {}}
	var marshalError *json.UnsupportedTypeError
	if _, err := repo.SyncTemplate(ctx, invalid); !errors.As(err, &marshalError) {
		t.Fatalf("marshal validation order changed: %v", err)
	}
	invalid = item
	invalid.Source.InputSchemaJSON = map[string]any{"bad": func() {}}
	if _, err := repo.SyncTemplate(t.Context(), invalid); !errors.As(err, &marshalError) {
		t.Fatalf("same digest bypassed schema serialization: %v", err)
	}
	item.Owner = TemplateSourceInfo{Type: "plugin", PluginID: "other", LocalID: "card"}
	if changed, err := repo.SyncTemplate(t.Context(), item); err == nil || changed {
		t.Fatal("same digest bypassed owner validation")
	}
}
