package catalog

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestRuntimeAndDeadLetterUpdatesKeepCommandOwnership(t *testing.T) {
	t.Parallel()
	catalog := New([]plugins.Snapshot{{PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running", Commands: []plugins.Command{{Name: "echo", Aliases: []string{"say"}}}}})
	before := catalog.Commands()
	if _, err := catalog.SetRuntimeResult("fixture", "dead_letter", "plugin.internal_error", "fixture failure"); err != nil {
		t.Fatal(err)
	}
	result, err := catalog.SetDeadLetterSnapshot("fixture", plugins.DeadLetterSnapshot{EnteredAt: time.Now(), CrashCount: 3, LastErrorCode: "plugin.internal_error"})
	if err != nil {
		t.Fatal(err)
	}
	result.Commands[0].Aliases[0] = "mutated"
	result.DeadLetter.CrashCount = 99
	current, _ := catalog.Get("fixture")
	if current.DeadLetter.CrashCount != 3 || catalog.Commands()[0].Commands[0].Aliases[0] != "say" || before[0].Commands[0].Aliases[0] != "say" {
		t.Fatal("state result mutation changed a command or lifecycle snapshot")
	}
	result, err = catalog.SetRuntimeResult("fixture", "running", "", "")
	if err != nil || result.DeadLetter != nil || len(catalog.Commands()) != 1 || catalog.Commands()[0].Commands[0].Name != "echo" {
		t.Fatalf("runtime recovery changed command availability: %+v %v", result, err)
	}
}

func TestBatchDesiredStatesPublishesWholeCommandSets(t *testing.T) {
	const count = 16
	entries := make([]plugins.Snapshot, count)
	enabled, disabled := make(map[string]string), make(map[string]string)
	for index := range entries {
		id := fmt.Sprintf("fixture-%02d", index)
		entries[index] = plugins.Snapshot{PluginID: id, Valid: true, RegistrationState: "installed", DesiredState: "enabled", Commands: []plugins.Command{{Name: id}}}
		enabled[id], disabled[id] = "enabled", "disabled"
	}
	catalog := New(entries)
	before := catalog.Commands()
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Go(func() {
		<-start
		for range 100 {
			catalog.ApplyDesiredStates(disabled)
			catalog.ApplyDesiredStates(enabled)
		}
	})
	workers.Go(func() {
		<-start
		for range 1000 {
			commands := catalog.Commands()
			if len(commands) != 0 && len(commands) != count {
				t.Errorf("reader observed a partial desired-state batch: %d", len(commands))
				return
			}
			for index, entry := range commands {
				if entry.PluginID != entries[index].PluginID || entry.Commands[0].Name != entries[index].PluginID {
					t.Error("command declaration changed during batch update")
					return
				}
			}
		}
	})
	close(start)
	workers.Wait()
	if len(before) != count || before[0].Commands[0].Name != entries[0].PluginID {
		t.Fatal("batch update modified an older published snapshot")
	}
	for _, entry := range catalog.List() {
		if entry.DesiredState != "enabled" {
			t.Fatal("batch state update was lost")
		}
	}
}

func TestDisabledCommandRefreshAppearsWhenEnabled(t *testing.T) {
	t.Parallel()
	catalog := New([]plugins.Snapshot{{PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "disabled", Commands: []plugins.Command{{Name: "old"}}, ManifestCommands: []plugins.Command{{ID: "echo", TriggerType: "setting", SettingsKey: "triggers"}}, DefaultConfig: map[string]any{"triggers": []any{"old"}}}})
	if _, ok := catalog.RefreshCommands("fixture", map[string]any{"triggers": []any{"new"}}); !ok {
		t.Fatal("disabled plugin was lost")
	}
	if len(catalog.Commands()) != 0 {
		t.Fatal("disabled command became routable")
	}
	if _, err := catalog.SetDesiredState("fixture", "enabled"); err != nil {
		t.Fatal(err)
	}
	commands := catalog.Commands()
	if len(commands) != 1 || len(commands[0].Commands) != 1 || commands[0].Commands[0].Name != "new" {
		t.Fatalf("enable published stale declarations: %+v", commands)
	}
}
