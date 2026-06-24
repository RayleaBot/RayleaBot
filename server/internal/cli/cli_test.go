package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestResetAdminAllowsArgon2idSetupAfterReset(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "user.yaml")
	databasePath := filepath.Join(root, "data", "rayleabot.db")
	writeFile(t, configPath, "schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n")

	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	repository, err := auth.NewSQLiteRepository(store)
	if err != nil {
		_ = store.Close()
		t.Fatalf("create auth repository: %v", err)
	}
	manager, err := auth.NewManager(auth.Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, auth.WithRepository(repository))
	if err != nil {
		_ = store.Close()
		t.Fatalf("create auth manager: %v", err)
	}
	if _, _, err := manager.Bootstrap("admin", "fixture-only-secret"); err != nil {
		_ = store.Close()
		t.Fatalf("bootstrap first admin: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close initial store: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if code := runResetAdmin(Command{ConfigPath: configPath, Logger: logger}); code != 0 {
		t.Fatalf("runResetAdmin exit code = %d", code)
	}

	resetStore, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("reopen sqlite database: %v", err)
	}
	defer resetStore.Close()

	resetRepository, err := auth.NewSQLiteRepository(resetStore)
	if err != nil {
		t.Fatalf("create reset auth repository: %v", err)
	}
	resetManager, err := auth.NewManager(auth.Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, auth.WithRepository(resetRepository))
	if err != nil {
		t.Fatalf("create reset auth manager: %v", err)
	}
	if _, _, err := resetManager.Bootstrap("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("bootstrap after reset failed: %v", err)
	}

	var digest []byte
	if err := resetStore.Read.QueryRow(`SELECT secret_digest FROM auth_bootstrap_state WHERE singleton_id = 1`).Scan(&digest); err != nil {
		t.Fatalf("load reset bootstrap digest: %v", err)
	}
	if !strings.HasPrefix(string(digest), "raylea-pwd:v2:argon2id:m=65536,t=3,p=1:") {
		t.Fatalf("expected reset bootstrap digest to use argon2id, got %q", string(digest))
	}
}

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
	createTestSQLiteDatabase(t, filepath.Join(dataDir, "rayleabot.db"))
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
	extractedDB := filepath.Join(t.TempDir(), "rayleabot.db")
	extractZipEntry(t, reader.File, "data/rayleabot.db", extractedDB)
	if err := storage.QuickCheckPath(t.Context(), extractedDB); err != nil {
		t.Fatalf("backup database quick_check failed: %v", err)
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
	createTestSQLiteDatabase(t, filepath.Join(dataDir, "rayleabot.db"))

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
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	mw, err := w.Create("backup-manifest.json")
	if err != nil {
		t.Fatalf("create manifest entry: %v", err)
	}
	if _, err := mw.Write(data); err != nil {
		t.Fatalf("write manifest entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := outFile.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

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
		DBSchemaVersion:     "000005",
		Consistency:         "offline",
		Directories: []recovery.BackupManifestDirectory{
			{Label: "config", Path: "config/user.yaml"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	mw, err := w.Create("backup-manifest.json")
	if err != nil {
		t.Fatalf("create manifest entry: %v", err)
	}
	if _, err := mw.Write(data); err != nil {
		t.Fatalf("write manifest entry: %v", err)
	}
	fw, err := w.Create("config/user.yaml")
	if err != nil {
		t.Fatalf("create config entry: %v", err)
	}
	if _, err := fw.Write([]byte("server:\n  host: 127.0.0.1\n")); err != nil {
		t.Fatalf("write config entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := outFile.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

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
	fw, err := w.Create("some-file.txt")
	if err != nil {
		t.Fatalf("create archive entry: %v", err)
	}
	if _, err := fw.Write([]byte("data")); err != nil {
		t.Fatalf("write archive entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := outFile.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

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
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	mw, err := w.Create("backup-manifest.json")
	if err != nil {
		t.Fatalf("create manifest entry: %v", err)
	}
	if _, err := mw.Write(data); err != nil {
		t.Fatalf("write manifest entry: %v", err)
	}

	// Attempt path traversal.
	fw, err := w.Create("../../../etc/evil.txt")
	if err != nil {
		t.Fatalf("create traversal entry: %v", err)
	}
	if _, err := fw.Write([]byte("malicious")); err != nil {
		t.Fatalf("write traversal entry: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := outFile.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

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

func TestConfigInitNormalizeValidateCommands(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if code := Run(Command{Name: "config", ConfigPath: configPath, Logger: logger, Args: []string{"init"}}); code != 0 {
		t.Fatalf("config init exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(configPath), "default.yaml")); err != nil {
		t.Fatalf("config init should create default.yaml: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config init should create user.yaml: %v", err)
	}

	writeFile(t, configPath, "schema_version: \"2\"\nserver:\n  port: 9090\n")
	if code := Run(Command{Name: "config", ConfigPath: configPath, Logger: logger, Args: []string{"validate"}}); code != 0 {
		t.Fatalf("config validate exit code = %d, want 0", code)
	}
	beforeNormalize, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config before normalize: %v", err)
	}
	if !strings.Contains(string(beforeNormalize), "port: 9090") || strings.Contains(string(beforeNormalize), "host:") {
		t.Fatalf("validate should not normalize config, got:\n%s", beforeNormalize)
	}

	if code := Run(Command{Name: "config", ConfigPath: configPath, Logger: logger, Args: []string{"normalize"}}); code != 0 {
		t.Fatalf("config normalize exit code = %d, want 0", code)
	}
	normalized, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read normalized config: %v", err)
	}
	if !strings.Contains(string(normalized), "host: 127.0.0.1") {
		t.Fatalf("normalize should write canonical defaults, got:\n%s", normalized)
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
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if len(report.Issues) == 0 {
		t.Fatal("doctor report must include at least one issue when config is missing")
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

func TestDoctorReportChecksSQLiteIntegrity(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	databasePath := filepath.Join(repoRoot, "data", "rayleabot.db")
	writeFile(t, configPath, "schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n")
	createTestSQLiteDatabase(t, databasePath)

	healthy := BuildDoctorReport(Command{
		ConfigPath: configPath,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	assertDoctorSummary(t, healthy.Issues, "database.ok", "Database accessible")

	if err := os.WriteFile(databasePath, []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := BuildDoctorReport(Command{
		ConfigPath: configPath,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	issue := findDoctorIssue(corrupt.Issues, "database.ping_failed")
	if issue == nil {
		t.Fatalf("doctor report should flag corrupt database, got %#v", corrupt.Issues)
	}
	if issue.Severity != "error" || !strings.Contains(issue.Summary, "Database integrity check failed") {
		t.Fatalf("unexpected corrupt database issue: %#v", issue)
	}
	if !strings.Contains(issue.Remediation, "data/quarantine/") || !strings.Contains(issue.Remediation, "data/sqlite-snapshots/") {
		t.Fatalf("corrupt database remediation should mention recovery paths: %#v", issue)
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
	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")

	writeFile(t, configPath, "schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n")
	if err := os.MkdirAll(filepath.Join(repoRoot, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, manifestPath, `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "chromium-windows-x64",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "windows-x64",
      "sources": [
        {
          "url": "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip",
          "kind": "upstream"
        }
      ],
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
      "sources": [
        {
          "url": "TODO(v0.1-phase0)",
          "kind": "upstream"
        }
      ],
      "sha256": "TODO(v0.1-phase0)",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/python.exe"]
      }
    },
    {
      "id": "nodejs-windows-x64",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "windows-x64",
      "sources": [
        {
          "url": "https://nodejs.org/download/release/v24.14.0/node-v24.14.0-win-x64.zip",
          "kind": "upstream"
        }
      ],
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

func TestDoctorReportSummarizesManagedRuntimeBootstrapStates(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	platform := deps.CurrentPlatform()
	pythonID := "python-" + platform
	nodeID := "nodejs-" + platform

	writeFile(t, configPath, "schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n")
	if err := os.MkdirAll(filepath.Join(repoRoot, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repoRoot, ".deps", "manifest.json"), `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "`+pythonID+`",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "`+platform+`",
      "sources": [
        {
          "url": "https://example.invalid/python.tar.gz",
          "kind": "upstream"
        }
      ],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/python.exe"]
      }
    },
    {
      "id": "`+nodeID+`",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "`+platform+`",
      "sources": [
        {
          "url": "https://example.invalid/node.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "archive_format": "zip",
      "entrypoints": {
        "node": ["node/node.exe"],
        "npm": ["node/npm.cmd"]
      }
    }
  ]
}
`)
	writeFile(t, filepath.Join(repoRoot, ".deps", "store", pythonID, "3.12.13", "python", "python.exe"), "")
	writeFile(t, filepath.Join(repoRoot, "cache", "downloads", "runtime", nodeID+"-24.14.0.zip"), "")

	report := BuildDoctorReport(Command{
		ConfigPath: configPath,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	assertDoctorSummary(t, report.Issues, "runtime.python_managed_ready", "Python 运行环境已准备完成。")
	assertDoctorSummary(t, report.Issues, "runtime.node_managed_ready", "Node.js / npm 环境已下载，启动时会解压。")
	assertDoctorSummary(t, report.Issues, "runtime.npm_managed_ready", "npm 已下载，启动时会解压。")
}

func assertDoctorSummary(t *testing.T, issues []DoctorIssue, code, summary string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == code {
			if issue.Summary != summary {
				t.Fatalf("unexpected doctor summary for %s: got %q want %q", code, issue.Summary, summary)
			}
			return
		}
	}
	t.Fatalf("doctor issue %s not found in %#v", code, issues)
}

func findDoctorIssue(issues []DoctorIssue, code string) *DoctorIssue {
	for i := range issues {
		if issues[i].Code == code {
			return &issues[i]
		}
	}
	return nil
}

func createTestSQLiteDatabase(t *testing.T, path string) {
	t.Helper()
	store, err := storage.Open(path)
	if err != nil {
		t.Fatalf("open test sqlite database: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close test sqlite database: %v", err)
		}
	}()
	if _, err := store.Write.Exec(`INSERT INTO plugin_instances (plugin_id, desired_state, updated_at) VALUES ('backup-test', 'enabled', '2026-06-13T00:00:00Z')`); err != nil {
		t.Fatalf("seed test sqlite database: %v", err)
	}
}

func extractZipEntry(t *testing.T, files []*zip.File, name string, targetPath string) {
	t.Helper()
	for _, file := range files {
		if file.Name != name {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatal(err)
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", name, err)
		}
		defer reader.Close()
		out, err := os.Create(targetPath)
		if err != nil {
			t.Fatalf("create extracted entry %s: %v", targetPath, err)
		}
		defer out.Close()
		if _, err := io.Copy(out, reader); err != nil {
			t.Fatalf("extract zip entry %s: %v", name, err)
		}
		return
	}
	t.Fatalf("zip entry %s not found", name)
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
