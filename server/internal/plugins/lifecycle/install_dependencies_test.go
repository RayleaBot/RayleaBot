package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestInstallDependencyAdmission(t *testing.T) {
	for _, scenario := range []struct {
		name, requirement, sourceType string
		installed                     []plugins.Snapshot
		missing                       bool
	}{
		{name: "missing", requirement: "required", sourceType: "local_directory", missing: true},
		{name: "invalid", requirement: "required", sourceType: "local_directory", installed: []plugins.Snapshot{{PluginID: "base", RegistrationState: plugins.RegistrationStateInstalled}}, missing: true},
		{name: "unregistered", requirement: "required", sourceType: "local_directory", installed: []plugins.Snapshot{{PluginID: "base", Valid: true}}, missing: true},
		{name: "disabled", requirement: "required", sourceType: "local_directory", installed: []plugins.Snapshot{{PluginID: "base", Valid: true, RegistrationState: plugins.RegistrationStateInstalled, DesiredState: plugins.DesiredStateDisabled}}},
		{name: "recommended", requirement: "recommended", sourceType: "local_directory"},
		{name: "development", requirement: "required", sourceType: "development"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			service, _ := newInstallTestService(t, t.TempDir(), tasks.NewRegistry(), scenario.installed, &stubInstallRepository{}, installerDeps{})
			t.Cleanup(func() { _ = service.Close() })
			source := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "dependent"), "dependent")
			path := filepath.Join(source, "info.json")
			payload, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var manifest map[string]any
			if err := json.Unmarshal(payload, &manifest); err != nil {
				t.Fatal(err)
			}
			manifest["dependencies"] = []plugins.Dependency{{ID: "base", Requirement: scenario.requirement}}
			payload, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, payload, 0o644); err != nil {
				t.Fatal(err)
			}
			refreshInstallArtifact(t, source)
			candidate, err := service.prepareCandidate(t.Context(), plugins.InstallRequest{
				SourceType: scenario.sourceType, Source: source, ResolvedSourceType: "local_directory", ResolvedSource: source,
			})
			if scenario.missing {
				var missing *plugins.DependencyMissingError
				if !errors.As(err, &missing) || InstallErrorCode(err) != errorcodes.PluginDependencyMissing || !reflect.DeepEqual(missing.PluginIDs, []string{"base"}) {
					t.Fatalf("missing dependency admission = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			candidate.cleanup()
		})
	}
}
