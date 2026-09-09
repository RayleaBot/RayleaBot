package storage

import (
	"path/filepath"
	"testing"
)

func TestRenderTemplateMigrationDiscardsEditorHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`DROP TABLE render_templates`,
		`DELETE FROM schema_migrations WHERE version = 7`,
		`CREATE TABLE render_template_revisions (revision_id TEXT PRIMARY KEY, html TEXT)`,
		`CREATE TABLE render_template_states (template_id TEXT PRIMARY KEY, current_revision_id TEXT REFERENCES render_template_revisions(revision_id))`,
		`INSERT INTO render_template_revisions VALUES ('old', 'obsolete content')`,
		`INSERT INTO render_template_states VALUES ('old-template', 'old')`,
		`CREATE TABLE migration_probe (value TEXT)`,
		`INSERT INTO migration_probe VALUES ('keep')`,
	} {
		if _, err := store.Write.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var obsolete, current int
	if err := store.Read.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('render_template_states','render_template_revisions')`).Scan(&obsolete); err != nil {
		t.Fatal(err)
	}
	if err := store.Read.QueryRow(`SELECT COUNT(*) FROM render_templates`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	var retained string
	if err := store.Read.QueryRow(`SELECT value FROM migration_probe`).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if obsolete != 0 || current != 0 || retained != "keep" {
		t.Fatalf("obsolete=%d current=%d unrelated=%q", obsolete, current, retained)
	}
}
