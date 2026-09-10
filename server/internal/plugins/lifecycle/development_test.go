package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestDevelopmentSyncIsIdempotentAndPreservesDisabledState(t *testing.T) {
	registry := tasks.NewRegistry()
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, t.TempDir(), registry, nil, repository, installerDeps{})
	if err := os.Remove(filepath.Join(service.repoRoot, "build_info.json")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	}()
	source := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "artifact"), "development-fixture")
	refreshInstallArtifact(t, source)
	sync := func() (string, bool) {
		t.Helper()
		id, changed, err := service.SyncDevelopment(t.Context(), source, source)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			if task := waitForTaskCompletion(t, registry, id); task.Status != tasks.StatusSucceeded {
				t.Fatalf("task=%#v", task)
			}
		}
		return id, changed
	}
	_, changed := sync()
	if !changed {
		t.Fatal("first install skipped")
	}
	if state := repository.saved["development-fixture"]; state != plugins.DesiredStateEnabled {
		t.Fatalf("new state=%s", state)
	}
	initialMetadata := repository.packages["development-fixture"]
	id, changed := sync()
	if changed || id != "" || len(registry.List()) != 1 {
		t.Fatal("unchanged package created an installation")
	}
	if repository.packages["development-fixture"] != initialMetadata {
		t.Fatal("no-op changed installation metadata")
	}
	if err := repository.SaveDesiredState(context.Background(), "development-fixture", plugins.DesiredStateDisabled, time.Now()); err != nil {
		t.Fatal(err)
	}
	entries := catalog.List()
	entries[0].DesiredState = plugins.DesiredStateDisabled
	catalog.Replace(entries)
	setInstallSourcePluginVersion(t, source, "0.2.0")
	_, changed = sync()
	if !changed {
		t.Fatal("replacement skipped")
	}
	if state := repository.saved["development-fixture"]; state != plugins.DesiredStateDisabled {
		t.Fatalf("replacement enabled a disabled plugin: %s", state)
	}
	installed, _ := catalog.Get("development-fixture")
	if err := os.WriteFile(filepath.Join(installed.PackageRootPath, "LICENSE"), []byte("damaged"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, changed = sync()
	if !changed {
		t.Fatal("damaged installed output was trusted")
	}
}
