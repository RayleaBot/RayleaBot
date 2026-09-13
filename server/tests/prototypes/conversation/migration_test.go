package conversation

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDesignMigrationRollbackSnapshotAndSchemaEquivalence(t *testing.T) {
	legacy, err := os.ReadFile("testdata/schema-000001.sql")
	if err != nil {
		t.Fatal(err)
	}
	const alteration = "ALTER TABLE plugin_kv ADD COLUMN expires_at_ms INTEGER; CREATE INDEX idx_plugin_kv_expiry ON plugin_kv(expires_at_ms) WHERE expires_at_ms IS NOT NULL;"
	for _, fail := range []bool{false, true} {
		root := t.TempDir()
		path := filepath.Join(root, "source.db")
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, err = db.Exec(string(legacy)); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("INSERT INTO schema_metadata VALUES (1,'000001','2026-09-13T00:00:00Z'); INSERT INTO plugin_kv VALUES ('fixture','cursor','42',2,'2026-09-13T00:00:00Z')"); err != nil {
			t.Fatal(err)
		}
		// VACUUM INTO creates a consistent standalone copy, including WAL data.
		backup := filepath.Join(root, "pre-migration.db")
		if _, err = db.Exec("VACUUM INTO ?", backup); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(alteration); err != nil {
			t.Fatal(err)
		}
		if fail {
			if _, err = tx.Exec("CREATE TABLE plugin_kv (broken INTEGER)"); err == nil {
				t.Fatal("fault injection did not fail")
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(before) != string(after) {
				t.Fatal("failed migration changed original database")
			}
		} else {
			if _, err = tx.Exec("UPDATE schema_metadata SET version='000002'"); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			var value string
			var expiry sql.NullInt64
			if err = db.QueryRow("SELECT value_json,expires_at_ms FROM plugin_kv").Scan(&value, &expiry); err != nil || value != "42" || expiry.Valid {
				t.Fatal("legacy values or permanence changed")
			}
			// Normalize sqlite_master to compare the fresh and migrated paths.
			fresh, err := sql.Open("sqlite", filepath.Join(root, "fresh.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = fresh.Close() })
			freshSQL := strings.Replace(string(legacy), "PRIMARY KEY (plugin_id, key)", "expires_at_ms INTEGER,\n    PRIMARY KEY (plugin_id, key)", 1)
			if _, err = fresh.Exec(freshSQL + "CREATE INDEX idx_plugin_kv_expiry ON plugin_kv(expires_at_ms) WHERE expires_at_ms IS NOT NULL;"); err != nil {
				t.Fatal(err)
			}
			for _, db := range []*sql.DB{db, fresh} {
				var count int
				if err = db.QueryRow("SELECT count(*) FROM pragma_table_info('plugin_kv') WHERE name='expires_at_ms' AND type='INTEGER' AND dflt_value IS NULL AND [notnull]=0").Scan(&count); err != nil || count != 1 {
					t.Fatal("fresh/migrated column mismatch")
				}
				var index string
				if err = db.QueryRow("SELECT sql FROM sqlite_master WHERE name='idx_plugin_kv_expiry'").Scan(&index); err != nil || !strings.Contains(index, "WHERE expires_at_ms IS NOT NULL") {
					t.Fatal("expiry index missing")
				}
			}
			if !slices.Equal(schemaObjects(t, db), schemaObjects(t, fresh)) {
				t.Fatalf("fresh and migrated sqlite_master differ:\n%v\n%v", schemaObjects(t, db), schemaObjects(t, fresh))
			}
		}
		copyDB, err := sql.Open("sqlite", backup)
		if err != nil {
			t.Fatal(err)
		}
		var check, version string
		if err = copyDB.QueryRow("PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
			t.Fatal("pre-migration snapshot invalid")
		}
		if err = copyDB.QueryRow("SELECT version FROM schema_metadata").Scan(&version); err != nil || version != "000001" {
			t.Fatal("snapshot not usable by old core")
		}
		if err = copyDB.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func schemaObjects(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT type,name,tbl_name,coalesce(sql,'') FROM sqlite_master ORDER BY type,name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	whitespace := regexp.MustCompile(`\s+`)
	var result []string
	for rows.Next() {
		var kind, name, table, statement string
		if err := rows.Scan(&kind, &name, &table, &statement); err != nil {
			t.Fatal(err)
		}
		statement = whitespace.ReplaceAllString(statement, "")
		statement = strings.ReplaceAll(statement, "IFNOTEXISTS", "")
		result = append(result, kind+":"+name+":"+table+":"+statement)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
