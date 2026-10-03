package system

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

type legacySystemCatalog struct{ plugins.CatalogView }

func TestSystemPluginCountsTrackCatalogUpdatesAndLegacyViews(t *testing.T) {
	t.Parallel()
	registry := catalog.New([]plugins.Snapshot{
		{PluginID: "running", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running"},
		{PluginID: "failed", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "crashed"},
		{PluginID: "invalid", Valid: false, RegistrationState: "installed", RuntimeState: "running"},
		{PluginID: "removed", Valid: true, RegistrationState: "removed", RuntimeState: "running"},
	})
	for _, source := range []plugins.CatalogView{registry, legacySystemCatalog{CatalogView: registry}} {
		service := &Service{plugins: source}
		running, failed := service.pluginStateCounts()
		if running != 1 || failed != 1 || service.pluginCount() != 4 {
			t.Fatalf("unexpected counts: %d/%d/%d", running, failed, service.pluginCount())
		}
	}
	if _, err := registry.SetRuntimeState("failed", "running"); err != nil {
		t.Fatal(err)
	}
	service := &Service{plugins: registry}
	if running, failed := service.pluginStateCounts(); running != 2 || failed != 0 || service.pluginCount() != 4 {
		t.Fatalf("stale counters after runtime update: %d/%d", running, failed)
	}
	registry.Replace(nil)
	if running, failed := service.pluginStateCounts(); running != 0 || failed != 0 || service.pluginCount() != 0 {
		t.Fatal("replacement left stale plugin counts")
	}
}
