package plugincatalog

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"

	"pgregory.net/rapid"
)

// --- Property-Based Tests ---

// Feature: plugin-write-api, Property 3: 启用状态更新与正确响应
// Validates: Requirements 2.1, 7.2
func TestProperty_SetDesiredState_Enable(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.StringMatching("[a-z][a-z0-9_]{2,30}").Draw(t, "pluginID")
		name := rapid.StringMatching("[A-Za-z][A-Za-z0-9 ]{0,20}").Draw(t, "name")
		version := rapid.StringMatching("[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}").Draw(t, "version")

		catalog := New([]plugins.Snapshot{{
			PluginID:          id,
			Name:              name,
			Version:           version,
			RegistrationState: "installed",
			DesiredState:      "disabled",
		}})

		snap, err := catalog.SetDesiredState(id, "enabled")
		if err != nil {
			t.Fatalf("SetDesiredState(%q, enabled) error: %v", id, err)
		}
		if snap.DesiredState != "enabled" {
			t.Fatalf("plugins.Snapshot.DesiredState = %q, want %q", snap.DesiredState, "enabled")
		}
		if snap.PluginID != id {
			t.Fatalf("plugins.Snapshot.PluginID = %q, want %q", snap.PluginID, id)
		}
	})
}

// Feature: plugin-write-api, Property 4: 禁用状态更新与正确响应
// Validates: Requirements 3.1, 7.2
func TestProperty_SetDesiredState_Disable(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.StringMatching("[a-z][a-z0-9_]{2,30}").Draw(t, "pluginID")
		name := rapid.StringMatching("[A-Za-z][A-Za-z0-9 ]{0,20}").Draw(t, "name")
		version := rapid.StringMatching("[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}").Draw(t, "version")

		catalog := New([]plugins.Snapshot{{
			PluginID:          id,
			Name:              name,
			Version:           version,
			RegistrationState: "installed",
			DesiredState:      "enabled",
		}})

		snap, err := catalog.SetDesiredState(id, "disabled")
		if err != nil {
			t.Fatalf("SetDesiredState(%q, disabled) error: %v", id, err)
		}
		if snap.DesiredState != "disabled" {
			t.Fatalf("plugins.Snapshot.DesiredState = %q, want %q", snap.DesiredState, "disabled")
		}
		if snap.PluginID != id {
			t.Fatalf("plugins.Snapshot.PluginID = %q, want %q", snap.PluginID, id)
		}
	})
}

// Feature: plugin-write-api, Property 5: 不存在的插件返回 plugins.ErrPluginNotFound
// Validates: Requirements 7.3
func TestProperty_SetDesiredState_NotFound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.StringMatching("[a-z][a-z0-9_]{2,30}").Draw(t, "pluginID")

		catalog := New(nil)

		_, err := catalog.SetDesiredState(id, "enabled")
		if !errors.Is(err, plugins.ErrPluginNotFound) {
			t.Fatalf("SetDesiredState(%q, enabled) on empty catalog: got err=%v, want plugins.ErrPluginNotFound", id, err)
		}
	})
}

// Feature: plugin-write-api, Property 7: Catalog 并发安全
// Validates: Requirements 7.4
func TestProperty_Catalog_ConcurrentSafety(t *testing.T) {
	entries := make([]plugins.Snapshot, 10)
	for i := range entries {
		entries[i] = plugins.Snapshot{
			PluginID:          fmt.Sprintf("plugin_%d", i),
			Name:              fmt.Sprintf("Plugin %d", i),
			Version:           "1.0.0",
			RegistrationState: "installed",
			DesiredState:      "disabled",
		}
	}
	catalog := New(entries)

	var wg sync.WaitGroup
	const goroutines = 20

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("plugin_%d", idx%len(entries))

			switch idx % 3 {
			case 0:
				desired := "enabled"
				if idx%2 == 0 {
					desired = "disabled"
				}
				_, _ = catalog.SetDesiredState(id, desired)
			case 1:
				catalog.Get(id)
			case 2:
				catalog.List()
			}
		}(g)
	}

	wg.Wait()
}

// --- Unit Tests ---

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
	case <-time.After(100 * time.Millisecond):
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
				Name:          "我的运势",
				CommandSource: plugins.CommandSourceDynamic,
				DeclarationID: "fortune",
			}},
			DynamicCommands: []plugins.DynamicCommandDecl{{
				ID:          "fortune",
				SettingsKey: "trigger_commands",
				Description: "查看今日运势",
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
				Name:          "weather",
				CommandSource: plugins.CommandSourceManifest,
			}},
			ManifestCommands: []plugins.Command{{
				Name: "weather",
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
