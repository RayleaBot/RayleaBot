package catalog

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestCatalogConcurrentUpdatesPreserveMembershipAndSnapshots(t *testing.T) {
	const count = 10
	entries := make([]plugins.Snapshot, count)
	for i := range entries {
		entries[i] = plugins.Snapshot{PluginID: fmt.Sprintf("plugin_%d", i), Name: fmt.Sprintf("Plugin %d", i), RegistrationState: "installed", DesiredState: "disabled"}
	}
	catalog := New(entries)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := range entries {
		workers.Go(func() {
			<-start
			for n := range 40 {
				desired := "enabled"
				if n%2 != 0 {
					desired = "disabled"
				}
				updated, err := catalog.SetDesiredState(entries[i].PluginID, desired)
				if err != nil || updated.DesiredState != desired {
					t.Errorf("update = %#v, %v", updated, err)
					return
				}
			}
		})
		workers.Go(func() {
			<-start
			for range 40 {
				snapshots := catalog.List()
				if len(snapshots) != count {
					t.Errorf("membership changed: %d", len(snapshots))
					return
				}
				for index, snapshot := range snapshots {
					if snapshot.PluginID != entries[index].PluginID || snapshot.Name != entries[index].Name || (snapshot.DesiredState != "enabled" && snapshot.DesiredState != "disabled") {
						t.Errorf("inconsistent snapshot: %#v", snapshot)
						return
					}
					snapshots[index].Name = "caller-owned mutation"
				}
				snapshot, found := catalog.Get(entries[i].PluginID)
				if !found || snapshot.Name != entries[i].Name {
					t.Errorf("read lost identity: %#v, %v", snapshot, found)
					return
				}
			}
		})
	}
	close(start)
	workers.Wait()
	for _, entry := range catalog.List() {
		if entry.DesiredState != "disabled" {
			t.Errorf("final write lost: %#v", entry)
		}
	}
}

// Validates: Requirements 7.3
func TestSetDesiredState_NotFound(t *testing.T) {
	catalog := New(nil)

	_, err := catalog.SetDesiredState("nonexistent_plugin", "enabled")
	if !errors.Is(err, plugins.ErrPluginNotFound) {
		t.Fatalf("got err=%v, want plugins.ErrPluginNotFound", err)
	}
}

// Validates: Requirements 7.1, 7.2
func TestSetDesiredState_NotInstalled_Conflict(t *testing.T) {
	catalog := New([]plugins.Snapshot{{
		PluginID:          "removed_plugin",
		Name:              "Removed",
		Version:           "1.0.0",
		RegistrationState: "removed",
		DesiredState:      "disabled",
	}})

	_, err := catalog.SetDesiredState("removed_plugin", "enabled")
	if !errors.Is(err, plugins.ErrStateConflict) {
		t.Fatalf("got err=%v, want plugins.ErrStateConflict", err)
	}
}

// Validates: Requirements 7.1, 7.2
func TestSetDesiredState_AlreadyEnabled_Conflict(t *testing.T) {
	catalog := New([]plugins.Snapshot{{
		PluginID:          "my_plugin",
		Name:              "My Plugin",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "enabled",
	}})

	_, err := catalog.SetDesiredState("my_plugin", "enabled")
	if !errors.Is(err, plugins.ErrStateConflict) {
		t.Fatalf("got err=%v, want plugins.ErrStateConflict", err)
	}
}

// Validates: Requirements 7.1, 7.2
func TestSetDesiredState_AlreadyDisabled_Conflict(t *testing.T) {
	catalog := New([]plugins.Snapshot{{
		PluginID:          "my_plugin",
		Name:              "My Plugin",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "disabled",
	}})

	_, err := catalog.SetDesiredState("my_plugin", "disabled")
	if !errors.Is(err, plugins.ErrStateConflict) {
		t.Fatalf("got err=%v, want plugins.ErrStateConflict", err)
	}
}

func TestCatalogSubscribePublishesUpdatedSnapshot(t *testing.T) {
	catalog := New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "disabled",
	}})

	updates, unsubscribe := catalog.Subscribe(1)
	defer unsubscribe()

	if _, err := catalog.SetDesiredState("weather", "enabled"); err != nil {
		t.Fatalf("SetDesiredState returned error: %v", err)
	}

	select {
	case update := <-updates:
		if update.PluginID != "weather" {
			t.Fatalf("update.PluginID = %q, want weather", update.PluginID)
		}
		if update.DesiredState != "enabled" {
			t.Fatalf("update.DesiredState = %q, want enabled", update.DesiredState)
		}
		if update.DisplayState != "enabled" {
			t.Fatalf("update.DisplayState = %q, want enabled", update.DisplayState)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for catalog update")
	}
}

func TestCatalogSubscribeSkipsUnchangedRuntimeState(t *testing.T) {
	catalog := New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
	}})

	updates, unsubscribe := catalog.Subscribe(1)
	defer unsubscribe()

	if _, err := catalog.SetRuntimeState("weather", "running"); err != nil {
		t.Fatalf("SetRuntimeState returned error: %v", err)
	}

	select {
	case update := <-updates:
		t.Fatalf("unexpected update: %#v", update)
	default:
	}
}

func TestRefreshCommandsPublishesAllSnapshotsForConflictRecalculation(t *testing.T) {
	catalog := New([]plugins.Snapshot{
		{
			PluginID:          "fortune",
			Name:              "Fortune",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			Commands: []plugins.Command{{
				ID: "fortune", Name: "我的运势", DisplayName: "今日运势",
				TriggerType: "setting", SettingsKey: "trigger_commands",
			}},
			ManifestCommands: []plugins.Command{{
				ID: "fortune", DisplayName: "今日运势", TriggerType: "setting",
				SettingsKey: "trigger_commands", Description: "查看今日运势",
			}},
			DefaultConfig: map[string]any{
				"trigger_commands": []any{"我的运势"},
			},
		},
		{
			PluginID:          "weather",
			Name:              "Weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			Commands: []plugins.Command{{
				ID: "weather", Name: "weather", DisplayName: "天气",
				TriggerType: "exact", TriggerNames: []string{"weather"},
			}},
			ManifestCommands: []plugins.Command{{
				ID: "weather", DisplayName: "天气", TriggerType: "exact", TriggerNames: []string{"weather"},
			}},
		},
	})

	updates, unsubscribe := catalog.Subscribe(2)
	defer unsubscribe()

	snapshot, ok := catalog.RefreshCommands("fortune", map[string]any{
		"trigger_commands": []any{"weather"},
	})
	if !ok {
		t.Fatal("RefreshCommands returned ok=false")
	}
	if len(snapshot.Commands) != 1 || snapshot.Commands[0].Name != "weather" {
		t.Fatalf("unexpected refreshed commands: %#v", snapshot.Commands)
	}

	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case update := <-updates:
			seen[update.PluginID] = true
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for update %d", i+1)
		}
	}
	if !seen["fortune"] || !seen["weather"] {
		t.Fatalf("published plugin IDs = %#v, want fortune and weather", seen)
	}
}

func TestCatalogCommandsFollowMutations(t *testing.T) {
	t.Parallel()

	enabled := plugins.Snapshot{
		PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "enabled",
		Commands: []plugins.Command{{ID: "weather", Name: "weather", TriggerType: "exact", TriggerNames: []string{"weather"}}},
	}
	catalog := New([]plugins.Snapshot{enabled})

	before := catalog.Commands()
	if len(before) != 1 || before[0].PluginID != "weather" || len(before[0].Commands) != 1 {
		t.Fatalf("unexpected initial command index: %+v", before)
	}

	disabled := enabled
	disabled.DesiredState = "disabled"
	catalog.Replace([]plugins.Snapshot{disabled})
	if got := catalog.Commands(); len(got) != 0 {
		t.Fatalf("disabled plugin must leave the index, got %+v", got)
	}
	if len(before) != 1 {
		t.Fatal("a previously returned index must stay intact")
	}

	catalog.Replace([]plugins.Snapshot{enabled})
	if got := catalog.Commands(); len(got) != 1 {
		t.Fatalf("re-enabled plugin must return to the index, got %+v", got)
	}
}
