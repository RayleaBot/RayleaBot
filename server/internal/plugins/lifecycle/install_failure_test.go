package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestInstallRollbackFailuresRemainFailedAndRetainRecoveryFiles(t *testing.T) {
	for _, failedStage := range []string{"rollback_files", "rollback_finalize", "rollback_stop"} {
		t.Run(failedStage, func(t *testing.T) {
			t.Parallel()
			registry := tasks.NewRegistry()
			repoRoot := t.TempDir()
			service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
			t.Cleanup(func() { _ = service.Close() })
			initial := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "initial"), "weather")
			initialTask, err := acceptInspected(t, service, plugins.InstallRequest{SourceType: "local_directory", Source: initial})
			if err != nil {
				t.Fatal(err)
			}
			if got := waitForTaskCompletion(t, registry, initialTask); got.Status != tasks.StatusSucceeded {
				t.Fatalf("initial install: %#v", got)
			}
			next := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "next"), "weather")
			setInstallSourcePluginVersion(t, next, "0.2.0")
			inspection, err := service.Inspect(t.Context(), plugins.InstallRequest{SourceType: "local_directory", Source: next, ReplaceExisting: true})
			if err != nil {
				t.Fatal(err)
			}
			workdir := service.inspections[inspection.InspectionID].workingRoot
			resumeCount := 0
			service.SetAfterSuccess(func(context.Context, string) error { return context.Canceled })
			service.SetAfterRollback(func(context.Context, string) error {
				resumeCount++
				if failedStage == "rollback_finalize" {
					return errors.New("test-secret-rollback-callback")
				}
				return nil
			})
			service.deps.rename = func(source, target string) error {
				if failedStage == "rollback_files" && filepath.Base(source) == "previous" {
					return errors.New("test-secret-restore-path")
				}
				return os.Rename(source, target)
			}
			stopCount := 0
			service.SetBeforeReplace(func(context.Context, string) error {
				stopCount++
				if failedStage == "rollback_stop" && stopCount > 1 {
					return errors.New("test-secret-stop-process")
				}
				return nil
			})
			taskID, err := service.Accept(t.Context(), plugins.InstallAcceptance{InspectionID: inspection.InspectionID, PackageSHA256: inspection.PackageSHA256})
			if err != nil {
				t.Fatal(err)
			}
			got := waitForTaskCompletion(t, registry, taskID)
			assertOperationFailure(t, got, "rollback_failed", []string{"finalize", failedStage})
			if _, err := os.Stat(workdir); err != nil {
				t.Fatalf("rollback recovery directory removed: %v", err)
			}
			if failedStage != "rollback_finalize" {
				if version := readInstallManifestVersion(t, filepath.Join(workdir, "previous", "info.json")); version != "0.1.0" {
					t.Fatalf("retained backup version = %q", version)
				}
				if resumeCount != 0 {
					t.Fatal("attempted resume after package restoration failed")
				}
			} else if version := readInstallManifestVersion(t, filepath.Join(service.installedRoot, "weather", "info.json")); version != "0.1.0" {
				t.Fatalf("restored version = %q", version)
			}
		})
	}
}

func TestInstallCleanupFailureReportsCommitted(t *testing.T) {
	t.Parallel()
	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{
		removeAll: func(path string) error {
			if strings.HasPrefix(filepath.Base(path), ".plugin-install-") {
				return errors.New("test-secret-cleanup")
			}
			return os.RemoveAll(path)
		},
	})
	t.Cleanup(func() { _ = service.Close() })
	source := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "source"), "weather")
	taskID, err := acceptInspected(t, service, plugins.InstallRequest{SourceType: "local_directory", Source: source})
	if err != nil {
		t.Fatal(err)
	}
	assertOperationFailure(t, waitForTaskCompletion(t, registry, taskID), "committed", []string{"cleanup"})
	if _, exists := catalog.Get("weather"); !exists {
		t.Fatal("committed package missing from catalog")
	}
	if _, err := os.Stat(filepath.Join(service.installedRoot, "weather", "info.json")); err != nil {
		t.Fatalf("committed package missing: %v", err)
	}
}

func TestInstallRollbackPreservesPrimaryAndRestorationCauses(t *testing.T) {
	t.Parallel()
	primary := errors.New("primary")
	restore := errors.New("restore")
	service, _ := newInstallTestService(t, t.TempDir(), tasks.NewRegistry(), nil, &stubInstallRepository{}, installerDeps{})
	t.Cleanup(func() { _ = service.Close() })
	source := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "source"), "weather")
	inspection, err := service.Inspect(t.Context(), plugins.InstallRequest{SourceType: "local_directory", Source: source})
	if err != nil {
		t.Fatal(err)
	}
	entry := service.inspections[inspection.InspectionID]
	service.SetAfterSuccess(func(context.Context, string) error { return primary })
	service.SetAfterRollback(func(context.Context, string) error { return restore })
	err = service.runInstall(installJob{ctx: t.Context(), request: entry.request, inspection: entry})
	if !errors.Is(err, primary) || !errors.Is(err, restore) {
		t.Fatalf("lost primary or rollback cause: %v", err)
	}
}

func assertOperationFailure(t *testing.T, got tasks.Snapshot, state string, stages []string) {
	t.Helper()
	if got.Status != tasks.StatusFailed || got.Error == nil {
		t.Fatalf("expected failed task, got %#v", got)
	}
	if got.Error.Details["operation_state"] != state || !reflect.DeepEqual(got.Error.Details["failures"], stages) {
		t.Fatalf("error details = %#v, want %s %v", got.Error.Details, state, stages)
	}
	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "test-secret-") {
		t.Fatalf("task exposed raw cause: %s", payload)
	}
}
