package catalog

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestDisplaySnapshotsOwnFieldsAndPreserveFullCatalog(t *testing.T) {
	t.Parallel()
	entry := plugins.Snapshot{
		PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "dead_letter",
		DefaultConfig: map[string]any{"nested": map[string]any{"list": []any{"value"}}}, Services: []plugins.Service{{Name: "fixture", Methods: []string{"run"}}},
		ManifestCommands: []plugins.Command{{Name: "manifest"}}, ManifestCommandPrefixes: &plugins.ManifestCommandPrefixes{Dedicated: []string{"manifest"}}, RenderTemplates: []plugins.RenderTemplate{{Path: "template.json"}},
		Commands: []plugins.Command{{Name: "echo", Aliases: []string{"say"}, TriggerNames: []string{"echo"}}}, CommandPrefixes: plugins.CommandPrefixes{Dedicated: []string{"*"}},
		CommandGroups: []plugins.CommandGroup{{ID: "main", Commands: []string{"echo"}}}, SourceRoots: []string{"community"}, ConflictPaths: []string{"fixture/info.json"},
		Events: []string{"message"}, Keywords: []string{"fixture"}, Webhooks: []plugins.WebhookScope{{ID: "hook", SourceCIDRs: []string{"127.0.0.0/8"}}},
		Screenshots: []plugins.Screenshot{{Path: "preview.png"}}, ManagementUI: &plugins.ManagementUI{Entry: "index.html", Pages: []plugins.ManagementUIPage{{ID: "main", Label: "Main"}}},
		Help: &plugins.Help{Title: "Help"}, DeadLetter: &plugins.DeadLetterSnapshot{EnteredAt: time.Unix(100, 0), CrashCount: 2},
	}
	catalog := New([]plugins.Snapshot{entry})
	full, _ := catalog.Get("fixture")
	want := plugins.CloneSnapshot(full)
	want.DefaultConfig, want.Services, want.ManifestCommands, want.ManifestCommandPrefixes, want.RenderTemplates = nil, nil, nil, nil, nil
	views := plugins.ReadDisplaySnapshots(catalog)
	if len(views) != 1 || !reflect.DeepEqual(views[0], want) {
		t.Fatalf("display fields changed: %+v", views)
	}
	view := &views[0]
	view.Commands[0].Aliases[0], view.Commands[0].TriggerNames[0], view.CommandPrefixes.Dedicated[0] = "changed", "changed", "changed"
	view.CommandGroups[0].Commands[0], view.SourceRoots[0], view.ConflictPaths[0], view.Events[0], view.Keywords[0] = "changed", "changed", "changed", "changed", "changed"
	view.Webhooks[0].SourceCIDRs[0], view.Screenshots[0].Path, view.ManagementUI.Pages[0].Label, view.Help.Title, view.DeadLetter.CrashCount = "changed", "changed", "changed", "changed", 99
	got, ok := plugins.ReadDisplaySnapshot(catalog, "fixture")
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatal("mutating a display list changed the stored display snapshot")
	}
	got.Commands[0].Aliases[0], got.ManagementUI.Pages[0].Label = "changed again", "changed again"
	if current, _ := catalog.Get("fixture"); !reflect.DeepEqual(current, full) {
		t.Fatal("display reads modified fields needed by lifecycle or installation")
	}
	if current := catalog.List(); !reflect.DeepEqual(current, []plugins.Snapshot{full}) {
		t.Fatal("complete catalog list lost declarations")
	}
	if _, ok := catalog.DisplaySnapshot("missing"); ok {
		t.Fatal("missing plugin became present")
	}
}

func TestCatalogCountsPreserveLifecycleStateProjection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, registration, desired, runtime, code string
		valid                                      bool
		running, failed                            int
	}{
		{"running while disabling", "installed", "disabled", "running", "", true, 1, 0},
		{"removed runtime", "removed", "enabled", "running", "", true, 0, 0},
		{"invalid runtime", "installed", "enabled", "running", "", false, 0, 0},
		{"crashed", "installed", "enabled", "crashed", "", true, 0, 1},
		{"backoff", "installed", "enabled", "backoff", "", true, 0, 1},
		{"dead letter", "installed", "enabled", "dead_letter", "", true, 0, 1},
		{"initialization failure", "installed", "enabled", "stopped", "fixture.error", true, 0, 1},
		{"disabled stopped error", "installed", "disabled", "stopped", "fixture.error", true, 0, 0},
		{"shutdown in progress", "installed", "enabled", "stopping", errorcodes.PluginShutdownTimeout, true, 0, 0},
		{"enabled shutdown failure", "installed", "enabled", "stopped", errorcodes.PluginShutdownTimeout, true, 0, 1},
		{"disabled shutdown failure", "installed", "disabled", "stopped", errorcodes.PluginShutdownTimeout, true, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := New([]plugins.Snapshot{{PluginID: "fixture", Valid: test.valid, RegistrationState: test.registration, DesiredState: test.desired, RuntimeState: test.runtime, RuntimeErrorCode: test.code}})
			want := plugins.CatalogStateCounts{Total: 1, Running: test.running, Failed: test.failed}
			if got := plugins.ReadCatalogStateCounts(catalog); got != want || plugins.ReadCatalogCount(catalog) != 1 {
				t.Fatalf("counts=%+v, want %+v", got, want)
			}
		})
	}
}

func TestDisplayAndCountReadersObserveCompleteCatalogReplacements(t *testing.T) {
	entries := func(state string) []plugins.Snapshot {
		return []plugins.Snapshot{
			{PluginID: "a", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: state, Commands: []plugins.Command{{Name: state, Aliases: []string{state}}}},
			{PluginID: "b", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: state, Commands: []plugins.Command{{Name: state, Aliases: []string{state}}}},
		}
	}
	catalog := New(entries("running"))
	old := catalog.DisplaySnapshots()
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Go(func() {
		<-start
		for range 50 {
			catalog.Replace(entries("crashed"))
			catalog.Replace(entries("running"))
		}
	})
	workers.Go(func() {
		<-start
		for range 200 {
			views := catalog.DisplaySnapshots()
			if len(views) != 2 || views[0].PluginID != "a" || views[1].PluginID != "b" || views[0].RuntimeState != views[1].RuntimeState || views[0].Commands[0].Name != views[1].Commands[0].Name {
				t.Error("display reader observed a partial replacement")
				return
			}
			views[0].Commands[0].Aliases[0] = "caller mutation"
			counts := catalog.StateCounts()
			if counts.Total != 2 || !((counts.Running == 2 && counts.Failed == 0) || (counts.Running == 0 && counts.Failed == 2)) {
				t.Errorf("counter observed a partial replacement: %+v", counts)
				return
			}
		}
	})
	close(start)
	workers.Wait()
	if old[0].RuntimeState != "running" || old[0].Commands[0].Aliases[0] != "running" {
		t.Fatal("replacement mutated a prior display snapshot")
	}
}
