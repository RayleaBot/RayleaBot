package integration

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	backupsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/backup"
	"github.com/RayleaBot/RayleaBot/server/internal/operations/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"gopkg.in/yaml.v3"
)

const legacySecretKeyName = "platform.secret_encryption_key"

// A 000002 backup restored into an empty directory migrates on first startup:
// encrypted secrets become usable, so sessions issued before the backup stay
// valid and the administrator can log in again.
func TestLegacyBackupRestoreMigratesSecretsAndKeepsSessions(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	now := func() time.Time { return current }
	sourceRoot := t.TempDir()
	sourceConfig := writeRuntimeRootConfig(t, sourceRoot)
	databasePath := filepath.Join(sourceRoot, "data", "rayleabot.db")

	source := newPersistentTestApp(t, sourceConfig, now, "legacy-source")
	sessionToken := issueLoginToken(t, source)
	closePersistentTestApp(t, source)
	sealSecretsAsSchema000002(t, databasePath)

	backup, err := backupsvc.Create(context.Background(), backupsvc.Options{
		RepoRoot:       sourceRoot,
		ConfigPath:     sourceConfig,
		DatabasePath:   databasePath,
		Consistency:    "offline",
		CreateSnapshot: storage.CreateSnapshot,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("create legacy backup: %v", err)
	}
	if backup.Manifest.DBSchemaVersion != "000002" {
		t.Fatalf("backup database version = %q, want 000002", backup.Manifest.DBSchemaVersion)
	}

	targetConfig := filepath.Join(t.TempDir(), "config", "user.yaml")
	if _, err := recovery.Restore(context.Background(), recovery.RestoreOptions{ConfigPath: targetConfig, ArchivePath: backup.ArchivePath}); err != nil {
		t.Fatalf("restore legacy backup: %v", err)
	}

	restored := newPersistentTestApp(t, targetConfig, now, "legacy-restored")
	defer closePersistentTestApp(t, restored)
	metadata, err := restored.Storage().SchemaMetadata(context.Background())
	if err != nil || metadata.Version != "000004" {
		t.Fatalf("restored schema = %#v, %v; want 000004", metadata, err)
	}
	var legacyRows int
	if err := restored.Storage().Read.QueryRow(
		"SELECT COUNT(*) FROM secret_store WHERE key = ? OR CAST(value AS TEXT) LIKE 'raylea-secret:v1:%'", legacySecretKeyName,
	).Scan(&legacyRows); err != nil || legacyRows != 0 {
		t.Fatalf("legacy secret rows after migration = %d, %v", legacyRows, err)
	}

	server := newManagementTestServer(t, restored.Handler())
	defer server.Close()
	for label, token := range map[string]string{
		"session issued before backup": sessionToken,
		"login after restore":          testutil.IssueExistingBootstrapLoginToken(t, restored),
	} {
		request, err := http.NewRequest(http.MethodGet, server.URL+"/api/adapters", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatalf("%s request: %v", label, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", label, response.StatusCode, http.StatusOK)
		}
	}
}

func writeRuntimeRootConfig(t *testing.T, root string) string {
	t.Helper()
	fixture := loadConfigFixture(t, testutil.RepoPath(t, "fixtures", "config", "ok.minimal.json"))
	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatal(err)
	}
	input["database"].(map[string]any)["path"] = filepath.Join(root, "data", "rayleabot.db")
	payload, err := yaml.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"config", "data"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(root, "config", "user.yaml")
	if err := os.WriteFile(configPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

// sealSecretsAsSchema000002 rewrites a current database into the 000002 form:
// Drop statistics added by 000004; 000003 only changed secret values.
func sealSecretsAsSchema000002(t *testing.T, databasePath string) {
	t.Helper()
	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if _, err := store.Write.Exec(`DROP TABLE message_stats_offline; DROP TABLE message_stats_runs;
		DROP TABLE message_stats_hours; DROP TABLE message_stats_adapters; DROP TABLE message_stats_tracking;`); err != nil {
		t.Fatal(err)
	}

	rows, err := store.Write.Query("SELECT key, value FROM secret_store")
	if err != nil {
		t.Fatal(err)
	}
	secrets := map[string][]byte{}
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatal(err)
		}
		secrets[key] = value
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(secrets) == 0 {
		t.Fatal("source database has no secrets to seal")
	}

	key := bytes.Repeat([]byte{7}, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range secrets {
		nonce := make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			t.Fatal(err)
		}
		sealed := "raylea-secret:v1:" + base64.RawURLEncoding.EncodeToString(nonce) + ":" + base64.RawURLEncoding.EncodeToString(gcm.Seal(nil, nonce, value, nil))
		if _, err := store.Write.Exec("UPDATE secret_store SET value = ? WHERE key = ?", []byte(sealed), name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Write.Exec(
		"INSERT INTO secret_store (key, value, created_at, updated_at) VALUES (?, ?, '2026-09-14T09:00:00Z', '2026-09-14T09:00:00Z')", legacySecretKeyName, key,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write.Exec("UPDATE schema_metadata SET version = '000002'"); err != nil {
		t.Fatal(err)
	}
}
