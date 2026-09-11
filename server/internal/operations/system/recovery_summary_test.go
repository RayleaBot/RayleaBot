package system

import (
	"errors"
	"slices"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/operations/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

// newRecoveryReconcileService saves a restore summary that still needs its
// post-start checks and installs a plugin requiring a newer core than the
// restored target, so reconciliation has to skip it.
func newRecoveryReconcileService(t *testing.T, desired *testutil.DesiredStateRecorder) (*Service, *plugincatalog.Catalog, string) {
	t.Helper()

	repoRoot := t.TempDir()
	if err := recovery.SaveSummary(repoRoot, recovery.CompatibilitySummary{
		Status:                  "pending",
		Phase:                   "pre_restore",
		TargetCoreVersion:       "0.2.0",
		RequiresPostStartChecks: true,
	}); err != nil {
		t.Fatalf("save recovery summary: %v", err)
	}
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather-pro",
		Version:           "1.4.0",
		ManifestVersion:   recovery.PluginManifestVersion,
		ArtifactVersion:   recovery.PluginArtifactVersion,
		MinCoreVersion:    "0.3.0",
		ManifestPath:      "plugins/installed/weather-pro/info.json",
		SourceRoot:        "plugins/installed",
		RegistrationState: "installed",
		DesiredState:      "enabled",
	}})
	service, err := New(Deps{
		CurrentConfig:    func() config.Config { return config.Config{} },
		CurrentSummary:   func() config.Summary { return config.Summary{} },
		CurrentRepoRoot:  func() string { return repoRoot },
		Plugins:          catalog,
		PluginRepository: desired,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, catalog, repoRoot
}

func TestReconcileRecoverySummaryPersistsSkippedPluginsDisabled(t *testing.T) {
	t.Parallel()

	desired := &testutil.DesiredStateRecorder{}
	service, catalog, _ := newRecoveryReconcileService(t, desired)

	summary, err := service.reconcileRecoverySummary()
	if err != nil {
		t.Fatalf("reconcile recovery summary: %v", err)
	}
	if summary == nil || len(summary.SkippedPlugins) != 1 || summary.SkippedPlugins[0].PluginID != "weather-pro" {
		t.Fatalf("reconciled summary = %#v, want weather-pro skipped", summary)
	}
	if got := desired.Saves(); !slices.Equal(got, []string{"weather-pro:disabled"}) {
		t.Fatalf("desired state writes = %#v", got)
	}
	if snapshot, _ := catalog.Get("weather-pro"); snapshot.DesiredState != "disabled" {
		t.Fatalf("catalog desired state = %q, want disabled", snapshot.DesiredState)
	}
}

func TestReconcileRecoverySummaryKeepsPendingChecksWhenDisableFails(t *testing.T) {
	t.Parallel()

	desired := &testutil.DesiredStateRecorder{SaveErr: errors.New("database is locked")}
	service, catalog, repoRoot := newRecoveryReconcileService(t, desired)

	if _, err := service.reconcileRecoverySummary(); !errors.Is(err, desired.SaveErr) {
		t.Fatalf("reconcile error = %v, want the repository failure", err)
	}
	saved, err := recovery.LoadSummary(repoRoot)
	if err != nil || saved == nil || !saved.RequiresPostStartChecks {
		t.Fatalf("saved summary = %#v, err = %v; want post-start checks still pending", saved, err)
	}
	if snapshot, _ := catalog.Get("weather-pro"); snapshot.DesiredState != "enabled" {
		t.Fatalf("catalog desired state = %q, want enabled until the write succeeds", snapshot.DesiredState)
	}
}
