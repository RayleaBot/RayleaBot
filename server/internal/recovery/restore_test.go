package recovery

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type restoreFixtureEntry struct {
	name    string
	payload []byte
	mode    os.FileMode
}

func createRestoreFixture(t *testing.T, databasePath string, extra ...restoreFixtureEntry) string {
	t.Helper()
	source := t.TempDir()
	configPath := filepath.Join(source, "config", "user.yaml")
	if _, _, err := config.Init(configPath, ""); err != nil {
		t.Fatal(err)
	}
	document, err := config.LoadDocument(configPath, "")
	if err != nil {
		t.Fatal(err)
	}
	document["database"].(map[string]any)["path"] = databasePath
	configuration, err := config.MarshalDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(source, "snapshot.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write.Exec(`INSERT INTO plugin_kv (plugin_id, key, value_json, size_bytes, updated_at) VALUES ('fixture', 'cursor', '42', 2, '2026-09-10T00:00:00Z')`); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := BuildBackupManifest(source, "offline")
	manifest.Directories = []BackupManifestDirectory{Directory("config/user.yaml", "config"), Directory(restoredDatabaseEntry, "database"), Directory("data", "data")}
	metadata, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	entries := []restoreFixtureEntry{
		{"backup-manifest.json", metadata, 0o600},
		{"config/user.yaml", configuration, 0o600},
		{restoredDatabaseEntry, database, 0o600},
		{"data/state.json", []byte("new state"), 0o600},
	}
	entries = append(entries, extra...)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		header.SetMode(entry.mode)
		stream, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stream.Write(entry.payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(source, "backup.zip")
	if err := os.WriteFile(archivePath, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func TestRestoreUsesArchivedCustomDatabasePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, "custom/state.db")})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Committed || result.DatabaseRelocated || result.DatabasePath != filepath.Join(root, "custom", "state.db") {
		t.Fatalf("result = %#v", result)
	}
	assertRestoredDatabase(t, root, "custom/state.db")
	if _, err := os.Stat(filepath.Join(root, restoredDatabaseEntry)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database also written to default location: %v", err)
	}
}

func TestRestoreRelocatesAbsoluteDatabaseInsideTarget(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.db")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, sourcePath := range []string{outside, "/var/lib/source/state.db", `C:\source\state.db`, `\\server\share\state.db`} {
		t.Run(sourcePath, func(t *testing.T) {
			root := t.TempDir()
			result, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, sourcePath)})
			if err != nil {
				t.Fatal(err)
			}
			if !result.DatabaseRelocated {
				t.Fatalf("absolute source was not relocated: %#v", result)
			}
			assertRestoredDatabase(t, root, restoredDatabaseEntry)
			if got, err := os.ReadFile(outside); err != nil || string(got) != "outside sentinel" {
				t.Fatalf("outside file changed: %q %v", got, err)
			}
		})
	}
}

func assertRestoredDatabase(t *testing.T, root, relative string) {
	t.Helper()
	document, err := config.LoadDocument(filepath.Join(root, "config", "user.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := document["database"].(map[string]any)["path"]; got != relative {
		t.Fatalf("database config = %v, want %s", got, relative)
	}
	store, err := storage.Open(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var value string
	if err := store.Read.QueryRow(`SELECT value_json FROM plugin_kv WHERE plugin_id='fixture' AND key='cursor'`).Scan(&value); err != nil || value != "42" {
		t.Fatalf("restored database data = %q, %v", value, err)
	}
}

func TestRestoreRejectsUnsafeAndCollidingArchiveBeforeMutation(t *testing.T) {
	for _, entry := range []restoreFixtureEntry{
		{"../escape", []byte("evil"), 0o600},
		{"/absolute", []byte("evil"), 0o600},
		{"data/../escape", []byte("evil"), 0o600},
		{"data/state.json", []byte("duplicate"), 0o600},
		{"data/STATE.json", []byte("case collision"), 0o600},
		{"data/link", []byte("../../escape"), os.ModeSymlink | 0o777},
		{"build_info.json", []byte("outside declared roots"), 0o600},
		{"data/CON", []byte("device"), 0o600},
	} {
		t.Run(entry.name, func(t *testing.T) {
			root := t.TempDir()
			original := seedRestoreTarget(t, root)
			_, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry, entry)})
			if err == nil {
				t.Fatal("unsafe archive accepted")
			}
			assertRestoreOriginals(t, root, original)
		})
	}
}

func TestRestoreRejectsUnsafeRelativeDatabaseBeforeMutation(t *testing.T) {
	for _, relative := range []string{"../outside.db", "custom/../../outside.db", "config/other.yaml", "plugins/state.db", ".git/config", "state.db", ".hidden/state.db", "data/state.db:stream"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			original := seedRestoreTarget(t, root)
			if _, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, relative)}); err == nil {
				t.Fatal("unsafe relative database accepted")
			}
			assertRestoreOriginals(t, root, original)
		})
	}
}

func TestRestoreRejectsTargetSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "state.json"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "data")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)}); err == nil {
		t.Fatal("target symlink accepted")
	}
	if got, err := os.ReadFile(filepath.Join(outside, "state.json")); err != nil || string(got) != "outside" {
		t.Fatalf("outside file changed: %q %v", got, err)
	}
}

func TestRestoreExtractionFailureAndCancellationLeaveTargetUntouched(t *testing.T) {
	for _, cancelCopy := range []bool{false, true} {
		t.Run(map[bool]string{false: "write failure", true: "cancellation"}[cancelCopy], func(t *testing.T) {
			root := t.TempDir()
			original := seedRestoreTarget(t, root)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			injected := errors.New("copy failed")
			_, err := restoreWithDeps(ctx, RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)}, restoreDeps{copy: func(ctx context.Context, _ *zip.File, output *os.File) error {
				if _, err := output.Write([]byte("partial")); err != nil {
					return err
				}
				if cancelCopy {
					cancel()
					return ctx.Err()
				}
				return injected
			}})
			if cancelCopy && !errors.Is(err, context.Canceled) || !cancelCopy && !errors.Is(err, injected) {
				t.Fatalf("copy cause lost: %v", err)
			}
			assertRestoreOriginals(t, root, original)
		})
	}
}

func TestRestoreRollsBackAllWritesOnFailureOrCancellation(t *testing.T) {
	for _, cancelCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "rename failure", true: "cancellation"}[cancelCommit], func(t *testing.T) {
			root := t.TempDir()
			original := seedRestoreTarget(t, root)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			injected := errors.New("commit failed")
			_, err := restoreWithDeps(ctx, RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)}, restoreDeps{rename: func(root *os.Root, old, new string) error {
				if strings.Contains(old, "incoming") && new == filepath.FromSlash("data/state.json") {
					if cancelCommit {
						if err := root.Rename(old, new); err != nil {
							return err
						}
						cancel()
						return nil
					}
					return injected
				}
				return root.Rename(old, new)
			}})
			if cancelCommit && !errors.Is(err, context.Canceled) || !cancelCommit && !errors.Is(err, injected) {
				t.Fatalf("commit cause lost: %v", err)
			}
			assertRestoreOriginals(t, root, original)
			if _, err := os.Stat(SummaryPath(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed restore published summary: %v", err)
			}
		})
	}
}

func TestRestoreRollbackFailureRetainsOriginalFilesAndBothCauses(t *testing.T) {
	root := t.TempDir()
	original := seedRestoreTarget(t, root)
	primary, rollback := errors.New("commit failed"), errors.New("rollback failed")
	_, err := restoreWithDeps(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)}, restoreDeps{rename: func(root *os.Root, old, new string) error {
		if strings.Contains(old, "incoming") && new == filepath.FromSlash("data/state.json") {
			return primary
		}
		if strings.Contains(old, "previous") && new == filepath.FromSlash("config/user.yaml") {
			return rollback
		}
		return root.Rename(old, new)
	}})
	var failure *RestoreError
	if !errors.Is(err, primary) || !errors.Is(err, rollback) || !errors.As(err, &failure) || failure.Stage != "rollback" || failure.RecoveryDirectory == "" {
		t.Fatalf("rollback outcome = %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(failure.RecoveryDirectory, "previous", "config", "user.yaml"))
	if err != nil || !bytes.Equal(payload, original["config/user.yaml"]) {
		t.Fatalf("original config was not retained at its relative path: %q %v", payload, err)
	}
}

func TestRestoreCRCFailureDoesNotReplaceExistingFile(t *testing.T) {
	archivePath := createRestoreFixture(t, restoredDatabaseEntry)
	payload, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range reader.File {
		if entry.Name == "data/state.json" {
			offset, err := entry.DataOffset()
			if err != nil {
				t.Fatal(err)
			}
			payload[offset] ^= 1
		}
	}
	if err := os.WriteFile(archivePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	original := seedRestoreTarget(t, root)
	if _, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: archivePath}); !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("CRC failure = %v", err)
	}
	assertRestoreOriginals(t, root, original)
}

func TestRestoreDatabaseLockConflictLeavesTargetUntouched(t *testing.T) {
	root := t.TempDir()
	store, err := storage.Open(filepath.Join(root, restoredDatabaseEntry))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	configPath := filepath.Join(root, "config", "user.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("original config"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Restore(t.Context(), RestoreOptions{ConfigPath: configPath, ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)})
	if !errors.Is(err, filelock.ErrLocked) {
		t.Fatalf("database lock was ignored: %v", err)
	}
	if got, err := os.ReadFile(configPath); err != nil || string(got) != "original config" {
		t.Fatalf("config changed before database admission: %q %v", got, err)
	}
}

func TestRestoreCleanupFailureReportsCommittedFilesAndRetainsWorkspace(t *testing.T) {
	root := t.TempDir()
	cleanupErr := errors.New("workspace cleanup failed")
	result, err := restoreWithDeps(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)}, restoreDeps{
		removeAll: func(*os.Root, string) error { return cleanupErr },
	})
	var failure *RestoreError
	if !result.Committed || !errors.Is(err, cleanupErr) || !errors.As(err, &failure) || failure.Stage != "cleanup" {
		t.Fatalf("committed cleanup outcome = %#v %v", result, err)
	}
	if _, err := os.Stat(failure.RecoveryDirectory); err != nil {
		t.Fatalf("cleanup recovery directory missing: %v", err)
	}
	assertRestoredDatabase(t, root, restoredDatabaseEntry)
}

func seedRestoreTarget(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{"config/user.yaml": []byte("original config"), "data/state.json": []byte("original state"), restoredDatabaseEntry: []byte("original database"), restoredDatabaseEntry + "-wal": []byte("original wal")}
	for name, payload := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func assertRestoreOriginals(t *testing.T, root string, original map[string][]byte) {
	t.Helper()
	names := make([]string, 0, len(original))
	for name := range original {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, original[name]) {
			t.Fatalf("original %s changed: %q %v", name, got, err)
		}
	}
}
