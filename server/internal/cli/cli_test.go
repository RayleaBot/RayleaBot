package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/recovery"
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
	writeFile(t, filepath.Join(dataDir, "rayleabot.db"), "fake-sqlite-data")
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
	if !names["data/rayleabot.db"] {
		t.Error("missing data/rayleabot.db in archive")
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
		var manifest recovery.BackupManifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			rc.Close()
			t.Fatalf("decode manifest: %v", err)
		}
		rc.Close()
		if manifest.Version != recovery.BackupManifestVersion {
			t.Errorf("manifest version = %q, want %s", manifest.Version, recovery.BackupManifestVersion)
		}
		if manifest.CoreVersion == "" {
			t.Error("manifest core_version should not be empty")
		}
		if manifest.ConfigSchemaVersion == "" || manifest.DBSchemaVersion == "" {
			t.Fatalf("manifest schema versions should not be empty: %#v", manifest)
		}
		if len(manifest.Directories) == 0 {
			t.Error("manifest directories should not be empty")
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
	writeFile(t, filepath.Join(dataDir, "rayleabot.db"), "fake-sqlite-data")

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
	restoredDB := filepath.Join(destDir, "data", "rayleabot.db")
	if _, err := os.Stat(restoredDB); err != nil {
		t.Errorf("restored database not found: %v", err)
	}

	summary, err := recovery.LoadSummary(destDir)
	if err != nil {
		t.Fatalf("load recovery summary: %v", err)
	}
	if summary == nil || summary.Status != "pending" {
		t.Fatalf("restore should persist pending recovery summary, got %#v", summary)
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
	manifest := recovery.BackupManifest{Version: "99", CreatedAt: "2025-01-01T00:00:00Z"}
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

func TestRestoreBlocksNewerDatabaseSchemaBeforeExtraction(t *testing.T) {
	t.Parallel()

	destDir := t.TempDir()
	archivePath := filepath.Join(t.TempDir(), "blocked.zip")
	outFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(outFile)
	manifest := recovery.BackupManifest{
		Version:             recovery.BackupManifestVersion,
		CreatedAt:           "2026-04-02T00:00:00Z",
		CoreVersion:         "0.2.0",
		ConfigSchemaVersion: "2",
		DBSchemaVersion:     "9999",
		Consistency:         "offline",
		Directories: []recovery.BackupManifestDirectory{
			{Label: "config", Path: "config/user.yaml"},
		},
	}
	data, _ := json.Marshal(manifest)
	mw, _ := w.Create("backup-manifest.json")
	mw.Write(data)
	fw, _ := w.Create("config/user.yaml")
	fw.Write([]byte("server:\n  host: 127.0.0.1\n"))
	w.Close()
	outFile.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := runRestore(Command{
		ConfigPath: filepath.Join(destDir, "config", "user.yaml"),
		Logger:     logger,
		Args:       []string{archivePath},
	})
	if code != 1 {
		t.Fatalf("restore should fail with exit code 1 for blocked compatibility, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(destDir, "config", "user.yaml")); err == nil {
		t.Fatal("restore should not extract files when compatibility is blocked")
	}
	summary, err := recovery.LoadSummary(destDir)
	if err != nil {
		t.Fatalf("load recovery summary: %v", err)
	}
	if summary == nil || summary.Status != "blocked" {
		t.Fatalf("restore should persist blocked recovery summary, got %#v", summary)
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

	manifest := recovery.BackupManifest{Version: recovery.BackupManifestVersion, CreatedAt: "2025-01-01T00:00:00Z"}
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

func TestDoctorReportIncludesStructuredIssues(t *testing.T) {
	t.Parallel()

	report := BuildDoctorReport(Command{
		ConfigPath: filepath.Join(t.TempDir(), "config", "user.yaml"),
		SchemaPath: filepath.Join(t.TempDir(), "contracts", "config.user.schema.json"),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if len(report.Issues) == 0 {
		t.Fatal("doctor report must include at least one issue when config and schema are missing")
	}

	for _, issue := range report.Issues {
		if issue.Code == "" || issue.Severity == "" || issue.Summary == "" {
			t.Fatalf("doctor issue must be fully populated: %#v", issue)
		}
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal doctor report: %v", err)
	}

	var decoded map[string]any
	if err := json.NewDecoder(bytes.NewReader(encoded)).Decode(&decoded); err != nil {
		t.Fatalf("decode doctor report: %v", err)
	}

	issues, ok := decoded["issues"].([]any)
	if !ok || len(issues) == 0 {
		t.Fatalf("encoded doctor report must expose issues: %#v", decoded)
	}
}

func TestDoctorReportIncludesRecoverySummaryWhenPresent(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := recovery.SaveSummary(repoRoot, recovery.CompatibilitySummary{
		Status:    "degraded",
		Phase:     "post_startup",
		Operation: "upgrade",
		CreatedAt: "2026-04-02T00:00:00Z",
		UpdatedAt: "2026-04-02T00:01:00Z",
	}); err != nil {
		t.Fatalf("save recovery summary: %v", err)
	}

	report := BuildDoctorReport(Command{
		ConfigPath: configPath,
		SchemaPath: filepath.Join(repoRoot, "contracts", "config.user.schema.json"),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if report.RecoverySummary == nil || report.RecoverySummary.Status != "degraded" {
		t.Fatalf("doctor report should expose recovery summary, got %#v", report.RecoverySummary)
	}
}

func TestDoctorReportFlagsIncompleteRuntimeMetadata(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	schemaPath := filepath.Join(repoRoot, "contracts", "config.user.schema.json")
	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")

	writeFile(t, configPath, "schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n")
	writeFile(t, schemaPath, "{}\n")
	if err := os.MkdirAll(filepath.Join(repoRoot, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, manifestPath, `{
  "manifest_version": 2,
  "resources": [
    {
      "id": "chromium-windows-x64",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "windows-x64",
      "source": "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip",
      "sha256": "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
      "archive_format": "zip",
      "entrypoints": {
        "browser": ["chrome-win64/chrome.exe"]
      }
    },
    {
      "id": "python-windows-x64",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "windows-x64",
      "source": "TODO(v0.1-phase0)",
      "sha256": "TODO(v0.1-phase0)",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/install/python.exe"],
        "pip": ["python/install/Scripts/pip.exe"]
      }
    },
    {
      "id": "nodejs-windows-x64",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "windows-x64",
      "source": "https://nodejs.org/download/release/v24.14.0/node-v24.14.0-win-x64.zip",
      "sha256": "deadbeef",
      "archive_format": "zip",
      "entrypoints": {
        "node": ["node-v24.14.0-win-x64/node.exe"],
        "npm": ["node-v24.14.0-win-x64/npm.cmd"]
      }
    }
  ]
}
`)

	report := BuildDoctorReport(Command{
		ConfigPath: configPath,
		SchemaPath: schemaPath,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	foundPython := false
	foundNode := false
	for _, issue := range report.Issues {
		switch issue.Code {
		case "deps.python_runtime_metadata_incomplete":
			foundPython = true
		case "deps.nodejs_runtime_metadata_incomplete":
			foundNode = true
		}
	}

	if !foundPython {
		t.Fatalf("doctor report should flag incomplete Python runtime metadata, got %#v", report.Issues)
	}
	if !foundNode {
		t.Fatalf("doctor report should flag incomplete Node.js runtime metadata, got %#v", report.Issues)
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
