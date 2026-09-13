package recovery

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
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
			_, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry, entry)})
			if err == nil {
				t.Fatal("unsafe archive accepted")
			}
			assertRestoreTargetsAbsent(t, root)
		})
	}
}

func TestRestoreRejectsUnsafeRelativeDatabaseBeforeMutation(t *testing.T) {
	for _, relative := range []string{"../outside.db", "custom/../../outside.db", "config/other.yaml", "plugins/state.db", ".git/config", "state.db", ".hidden/state.db", "data/state.db:stream"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			if _, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, relative)}); err == nil {
				t.Fatal("unsafe relative database accepted")
			}
			assertRestoreTargetsAbsent(t, root)
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

func TestRestoreRejectsExistingTargetsWithoutWriting(t *testing.T) {
	for _, existing := range []string{"config/user.yaml", "data/state.json", restoredDatabaseEntry, restoredDatabaseEntry + "-wal", RecoverySummaryPath} {
		t.Run(existing, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, filepath.FromSlash(existing))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("existing"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)})
			var failure *RestoreError
			if !errors.As(err, &failure) || failure.Stage != "target" {
				t.Fatalf("existing %s accepted: %v", existing, err)
			}
			if got, err := os.ReadFile(target); err != nil || string(got) != "existing" {
				t.Fatalf("existing %s changed: %q %v", existing, got, err)
			}
			for _, name := range restoreTargets {
				if name == existing {
					continue
				}
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("restore wrote %s despite existing %s: %v", name, existing, err)
				}
			}
		})
	}
}

func TestRestoreExtractionFailureAndCancellationWriteNothing(t *testing.T) {
	for _, cancelCopy := range []bool{false, true} {
		t.Run(map[bool]string{false: "write failure", true: "cancellation"}[cancelCopy], func(t *testing.T) {
			root := t.TempDir()
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
			assertRestoreTargetsAbsent(t, root)
		})
	}
}

func TestRestoreRemovesWrittenFilesOnFailureOrCancellation(t *testing.T) {
	for _, cancelCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "rename failure", true: "cancellation"}[cancelCommit], func(t *testing.T) {
			root := t.TempDir()
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
			assertRestoreTargetsAbsent(t, root)
		})
	}
}

func TestRestoreRemovalFailureReportsBothCauses(t *testing.T) {
	root := t.TempDir()
	primary, removal := errors.New("commit failed"), errors.New("remove failed")
	_, err := restoreWithDeps(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)}, restoreDeps{
		rename: func(root *os.Root, old, new string) error {
			if strings.Contains(old, "incoming") && new == filepath.FromSlash("data/state.json") {
				return primary
			}
			return root.Rename(old, new)
		},
		remove: func(root *os.Root, name string) error {
			if name == filepath.FromSlash("config/user.yaml") {
				return removal
			}
			return root.Remove(name)
		},
	})
	var failure *RestoreError
	if !errors.Is(err, primary) || !errors.Is(err, removal) || !errors.As(err, &failure) || failure.Stage != "remove" {
		t.Fatalf("removal outcome = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "config", "user.yaml")); err != nil {
		t.Fatalf("file that could not be removed disappeared: %v", err)
	}
}

func TestRestoreCRCFailureWritesNothing(t *testing.T) {
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
	if _, err := Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: archivePath}); !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("CRC failure = %v", err)
	}
	assertRestoreTargetsAbsent(t, root)
}

func TestRestoreDatabaseLockConflictWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	lock, err := filelock.Acquire(filepath.Join(root, filepath.FromSlash(restoredDatabaseEntry)+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	_, err = Restore(t.Context(), RestoreOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), ArchivePath: createRestoreFixture(t, restoredDatabaseEntry)})
	if !errors.Is(err, filelock.ErrLocked) {
		t.Fatalf("database lock was ignored: %v", err)
	}
	assertRestoreTargetsAbsent(t, root)
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

// restoreTargets are the files the fixture archive restores into a target root.
var restoreTargets = []string{"config/user.yaml", "data/state.json", restoredDatabaseEntry, RecoverySummaryPath}

func assertRestoreTargetsAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range restoreTargets {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("restore left %s behind: %v", name, err)
		}
	}
}
