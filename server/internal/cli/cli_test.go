package cli

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupCreatesValidArchive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	dataDir := filepath.Join(dir, "data")
	pluginsDir := filepath.Join(dir, "plugins", "installed", "hello-python")
	backupsDir := filepath.Join(dir, "backups")

	for _, d := range []string{configDir, dataDir, pluginsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(t, filepath.Join(configDir, "user.yaml"), "server:\n  listen: 127.0.0.1:9600\n")
	writeFile(t, filepath.Join(dataDir, "state.db"), "fake-sqlite-data")
	writeFile(t, filepath.Join(pluginsDir, "info.json"), `{"id":"hello-python"}`)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runBackup(Command{
		ConfigPath: filepath.Join(configDir, "user.yaml"),
		Logger:     logger,
	})
	if code != 0 {
		t.Fatalf("backup returned exit code %d, want 0", code)
	}

	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		t.Fatalf("read backups dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup file, got %d", len(entries))
	}

	archivePath := filepath.Join(backupsDir, entries[0].Name())
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open backup archive: %v", err)
	}
	defer reader.Close()

	names := map[string]bool{}
	for _, f := range reader.File {
		names[f.Name] = true
	}

	if !names["backup-manifest.json"] {
		t.Error("missing backup-manifest.json in archive")
	}
	if !names["config/user.yaml"] {
		t.Error("missing config/user.yaml in archive")
	}
	if !names["data/state.db"] {
		t.Error("missing data/state.db in archive")
	}

	// Validate manifest structure.
	for _, f := range reader.File {
		if f.Name != "backup-manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open manifest: %v", err)
		}
		var manifest backupManifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			rc.Close()
			t.Fatalf("decode manifest: %v", err)
		}
		rc.Close()
		if manifest.Version != "1" {
			t.Errorf("manifest version = %q, want 1", manifest.Version)
		}
		if len(manifest.Items) == 0 {
			t.Error("manifest items should not be empty")
		}
	}
}

func TestRestoreExtractsArchiveContents(t *testing.T) {
	t.Parallel()

	// Create a backup archive to restore from.
	srcDir := t.TempDir()
	configDir := filepath.Join(srcDir, "config")
	dataDir := filepath.Join(srcDir, "data")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(configDir, "user.yaml"), "server:\n  listen: 127.0.0.1:9600\n")
	writeFile(t, filepath.Join(dataDir, "state.db"), "fake-sqlite-data")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runBackup(Command{
		ConfigPath: filepath.Join(configDir, "user.yaml"),
		Logger:     logger,
	})
	if code != 0 {
		t.Fatalf("backup returned exit code %d", code)
	}

	backupsDir := filepath.Join(srcDir, "backups")
	entries, err := os.ReadDir(backupsDir)
	if err != nil || len(entries) == 0 {
		t.Fatal("no backup file created")
	}
	archivePath := filepath.Join(backupsDir, entries[0].Name())

	// Restore into a fresh directory.
	destDir := t.TempDir()
	destConfigDir := filepath.Join(destDir, "config")
	if err := os.MkdirAll(destConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}

	restoreCode := runRestore(Command{
		ConfigPath: filepath.Join(destConfigDir, "user.yaml"),
		Logger:     logger,
		Args:       []string{archivePath},
	})
	if restoreCode != 0 {
		t.Fatalf("restore returned exit code %d, want 0", restoreCode)
	}

	// Verify restored files exist.
	restoredConfig := filepath.Join(destDir, "config", "user.yaml")
	if _, err := os.Stat(restoredConfig); err != nil {
		t.Errorf("restored config not found: %v", err)
	}
	restoredDB := filepath.Join(destDir, "data", "state.db")
	if _, err := os.Stat(restoredDB); err != nil {
		t.Errorf("restored database not found: %v", err)
	}
}

func TestRestoreRejectsInvalidManifestVersion(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "bad.zip")
	outFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(outFile)
	manifest := backupManifest{Version: "99", CreatedAt: "2025-01-01T00:00:00Z"}
	data, _ := json.Marshal(manifest)
	mw, _ := w.Create("backup-manifest.json")
	mw.Write(data)
	w.Close()
	outFile.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runRestore(Command{
		ConfigPath: filepath.Join(t.TempDir(), "config", "user.yaml"),
		Logger:     logger,
		Args:       []string{archivePath},
	})
	if code != 1 {
		t.Fatalf("restore should fail with exit code 1 for unsupported version, got %d", code)
	}
}

func TestRestoreRejectsMissingManifest(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "no-manifest.zip")
	outFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(outFile)
	fw, _ := w.Create("some-file.txt")
	fw.Write([]byte("data"))
	w.Close()
	outFile.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runRestore(Command{
		ConfigPath: filepath.Join(t.TempDir(), "config", "user.yaml"),
		Logger:     logger,
		Args:       []string{archivePath},
	})
	if code != 1 {
		t.Fatalf("restore should fail with exit code 1 for missing manifest, got %d", code)
	}
}

func TestRestoreRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "traversal.zip")
	outFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(outFile)

	manifest := backupManifest{Version: "1", CreatedAt: "2025-01-01T00:00:00Z"}
	data, _ := json.Marshal(manifest)
	mw, _ := w.Create("backup-manifest.json")
	mw.Write(data)

	// Attempt path traversal.
	fw, _ := w.Create("../../../etc/evil.txt")
	fw.Write([]byte("malicious"))

	w.Close()
	outFile.Close()

	destDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runRestore(Command{
		ConfigPath: filepath.Join(destDir, "config", "user.yaml"),
		Logger:     logger,
		Args:       []string{archivePath},
	})
	// Should succeed but skip the traversal entry.
	if code != 0 {
		t.Fatalf("restore should succeed (skipping traversal), got exit code %d", code)
	}

	// The evil file should NOT exist outside the dest dir.
	evilPath := filepath.Join(destDir, "..", "..", "..", "etc", "evil.txt")
	if _, err := os.Stat(evilPath); err == nil {
		t.Fatal("path traversal entry should have been skipped")
	}
}

func TestRestoreRequiresBackupPath(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runRestore(Command{
		ConfigPath: filepath.Join(t.TempDir(), "config", "user.yaml"),
		Logger:     logger,
		Args:       []string{},
	})
	if code != 1 {
		t.Fatalf("restore should fail with exit code 1 when no path given, got %d", code)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
