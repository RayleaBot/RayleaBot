package system

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestSystemBackupUsesSQLiteSnapshotAndPreservesTaskShape(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")
	databasePath := filepath.Join(repoRoot, "data", "rayleabot.db")
	writeAppTestFile(t, configPath, "schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n")

	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer store.Close()
	if _, err := store.Write.Exec(`INSERT INTO plugin_instances (plugin_id, desired_state, updated_at) VALUES ('system-backup-test', 'enabled', '2026-06-13T00:00:00Z')`); err != nil {
		t.Fatalf("seed sqlite store: %v", err)
	}

	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, 5*time.Second)
	defer executor.Close()

	system := New(Deps{
		CurrentConfig: func() config.Config {
			return config.Config{
				Database: config.DatabaseConfig{Path: filepath.Join("data", "rayleabot.db")},
			}
		},
		CurrentSummary: func() config.Summary {
			return config.Summary{
				ConfigPath: configPath,
			}
		},
		CurrentRepoRoot: func() string { return repoRoot },
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		Storage:         store,
		TaskExecutor:    executor,
	})
	handler := systemapi.NewHandlers(system).HandleSystemBackup()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/system/backup", nil))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("backup response status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	var accepted taskAcceptedResponse
	if err := json.NewDecoder(recorder.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode backup response: %v", err)
	}
	if accepted.TaskID == "" {
		t.Fatalf("backup response should include task_id: %#v", accepted)
	}

	snapshot := waitTask(t, registry, accepted.TaskID, tasks.StatusSucceeded)
	if snapshot.Result == nil {
		t.Fatalf("backup task should expose result: %#v", snapshot)
	}
	archivePath, _ := snapshot.Result.Details["archive_path"].(string)
	if archivePath == "" {
		t.Fatalf("backup task result should expose archive_path: %#v", snapshot.Result)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open backup archive: %v", err)
	}
	defer reader.Close()

	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	if !names["backup-manifest.json"] || !names["config/user.yaml"] || !names["data/rayleabot.db"] {
		t.Fatalf("backup archive shape changed: %#v", names)
	}

	extractedDB := filepath.Join(t.TempDir(), "rayleabot.db")
	extractAppZipEntry(t, reader.File, "data/rayleabot.db", extractedDB)
	if err := storage.QuickCheckPath(t.Context(), extractedDB); err != nil {
		t.Fatalf("backup database quick_check failed: %v", err)
	}
}

func writeAppTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func extractAppZipEntry(t *testing.T, files []*zip.File, name string, targetPath string) {
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
