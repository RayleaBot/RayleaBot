package storage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func createLegacyDatabase(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/schema-000001.sql")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(string(data)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO schema_metadata VALUES (1,'000001','2026-09-13T00:00:00Z'); INSERT INTO plugin_kv VALUES ('fixture','cursor','42',2,'2026-09-13T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenMigratesLegacyAndPreservesBusinessData(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	metadata, err := store.SchemaMetadata(t.Context())
	if err != nil || metadata.Version != "000008" || metadata.InitializedAt != "2026-09-13T00:00:00Z" {
		t.Fatalf("metadata changed: %#v %v", metadata, err)
	}
	var value string
	var expiry sql.NullInt64
	if err := store.Read.QueryRow("SELECT value_json,expires_at_ms FROM plugin_kv WHERE plugin_id='fixture'").Scan(&value, &expiry); err != nil || value != "42" || expiry.Valid {
		t.Fatalf("legacy value changed: %q %v %v", value, expiry, err)
	}
	fresh := openTestStore(t)
	if !reflect.DeepEqual(migrationSchemaSQL(t, store.Read), migrationSchemaSQL(t, fresh.Read)) {
		t.Fatal("fresh and migrated sqlite_master differ")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if next, err := second.SchemaMetadata(t.Context()); err != nil || next != metadata {
		t.Fatal("repeated startup changed metadata")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func migrationSchemaSQL(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query("SELECT type,name,coalesce(sql,'') FROM sqlite_master ORDER BY type,name")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	result := make(map[string]string)
	for rows.Next() {
		var kind, name, statement string
		if err := rows.Scan(&kind, &name, &statement); err != nil {
			t.Fatal(err)
		}
		result[kind+":"+name] = strings.ReplaceAll(strings.Join(strings.Fields(statement), ""), "IFNOTEXISTS", "")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestOpenFailedMigrationDoesNotChangeLegacyDatabase(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	// The index conflict fails after ADD COLUMN, exercising DDL rollback.
	if _, err := db.Exec("CREATE INDEX idx_plugin_kv_expiry ON plugin_kv(key)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if store, err := Open(path); err == nil {
		_ = store.Close()
		t.Fatal("faulty migration succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed startup changed original database")
	}
}

func TestMigrationUnknownVersionDoesNotWrite(t *testing.T) {
	t.Parallel()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("UPDATE schema_metadata SET version='999999'"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if store, err := Open(path); err == nil {
		_ = store.Close()
		t.Fatal("unknown version accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("unknown version changed database")
	}
}

func sealLegacySecret(t *testing.T, key []byte, plaintext string) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return []byte(legacySealedSecretPrefix + base64.RawURLEncoding.EncodeToString(nonce) + ":" + base64.RawURLEncoding.EncodeToString(ciphertext))
}

func createLegacySecretDatabase(t *testing.T, secrets map[string][]byte) string {
	t.Helper()
	path := createLegacyDatabase(t)
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for key, value := range secrets {
		if _, err := db.Exec("INSERT INTO secret_store (key, value, created_at, updated_at) VALUES (?, ?, '2026-09-13T00:00:00Z', '2026-09-13T00:00:00Z')", key, value); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func readSecretRows(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query("SELECT key, value FROM secret_store")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	result := map[string]string{}
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatal(err)
		}
		result[key] = string(value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestOpenDecryptsLegacySecretsAndDropsKey(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte{7}, 32)
	path := createLegacySecretDatabase(t, map[string][]byte{
		legacySecretKeyName:           key,
		"plugin:fixture:secret:token": sealLegacySecret(t, key, "fixture-token"),
		"auth:signing":                []byte("raw-value"),
	})
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	want := map[string]string{"plugin:fixture:secret:token": "fixture-token", "auth:signing": "raw-value"}
	if got := readSecretRows(t, store.Read); !reflect.DeepEqual(got, want) {
		t.Fatalf("secrets after migration = %#v, want %#v", got, want)
	}
}

func TestOpenKeepsCiphertextWhenLegacySecretCannotBeDecrypted(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte{7}, 32)
	sealed := sealLegacySecret(t, bytes.Repeat([]byte{8}, 32), "fixture-token")
	path := createLegacySecretDatabase(t, map[string][]byte{legacySecretKeyName: key, "plugin:fixture:secret:token": sealed})
	if store, err := Open(path); err == nil {
		_ = store.Close()
		t.Fatal("undecryptable secret migrated")
	}
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var version string
	if err := db.QueryRow("SELECT version FROM schema_metadata").Scan(&version); err != nil || version != "000002" {
		t.Fatalf("version = %q %v, want 000002", version, err)
	}
	want := map[string]string{legacySecretKeyName: string(key), "plugin:fixture:secret:token": string(sealed)}
	if got := readSecretRows(t, db); !reflect.DeepEqual(got, want) {
		t.Fatal("failed migration changed stored secrets")
	}
}
