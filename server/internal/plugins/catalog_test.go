package plugins

import (
	"errors"
	"fmt"
	"sync"
	"testing"

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

		catalog := NewCatalog([]Snapshot{{
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
			t.Fatalf("Snapshot.DesiredState = %q, want %q", snap.DesiredState, "enabled")
		}
		if snap.PluginID != id {
			t.Fatalf("Snapshot.PluginID = %q, want %q", snap.PluginID, id)
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

		catalog := NewCatalog([]Snapshot{{
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
			t.Fatalf("Snapshot.DesiredState = %q, want %q", snap.DesiredState, "disabled")
		}
		if snap.PluginID != id {
			t.Fatalf("Snapshot.PluginID = %q, want %q", snap.PluginID, id)
		}
	})
}

// Feature: plugin-write-api, Property 5: 不存在的插件返回 ErrPluginNotFound
// Validates: Requirements 7.3
func TestProperty_SetDesiredState_NotFound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.StringMatching("[a-z][a-z0-9_]{2,30}").Draw(t, "pluginID")

		catalog := NewCatalog(nil)

		_, err := catalog.SetDesiredState(id, "enabled")
		if !errors.Is(err, ErrPluginNotFound) {
			t.Fatalf("SetDesiredState(%q, enabled) on empty catalog: got err=%v, want ErrPluginNotFound", id, err)
		}
	})
}

// Feature: plugin-write-api, Property 7: Catalog 并发安全
// Validates: Requirements 7.4
func TestProperty_Catalog_ConcurrentSafety(t *testing.T) {
	entries := make([]Snapshot, 10)
	for i := range entries {
		entries[i] = Snapshot{
			PluginID:          fmt.Sprintf("plugin_%d", i),
			Name:              fmt.Sprintf("Plugin %d", i),
			Version:           "1.0.0",
			RegistrationState: "installed",
			DesiredState:      "disabled",
		}
	}
	catalog := NewCatalog(entries)

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
	catalog := NewCatalog(nil)

	_, err := catalog.SetDesiredState("nonexistent_plugin", "enabled")
	if !errors.Is(err, ErrPluginNotFound) {
		t.Fatalf("got err=%v, want ErrPluginNotFound", err)
	}
}

// Validates: Requirements 7.1, 7.2
func TestSetDesiredState_NotInstalled_Conflict(t *testing.T) {
	catalog := NewCatalog([]Snapshot{{
		PluginID:          "removed_plugin",
		Name:              "Removed",
		Version:           "1.0.0",
		RegistrationState: "removed",
		DesiredState:      "disabled",
	}})

	_, err := catalog.SetDesiredState("removed_plugin", "enabled")
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("got err=%v, want ErrStateConflict", err)
	}
}

// Validates: Requirements 7.1, 7.2
func TestSetDesiredState_AlreadyEnabled_Conflict(t *testing.T) {
	catalog := NewCatalog([]Snapshot{{
		PluginID:          "my_plugin",
		Name:              "My Plugin",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "enabled",
	}})

	_, err := catalog.SetDesiredState("my_plugin", "enabled")
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("got err=%v, want ErrStateConflict", err)
	}
}

// Validates: Requirements 7.1, 7.2
func TestSetDesiredState_AlreadyDisabled_Conflict(t *testing.T) {
	catalog := NewCatalog([]Snapshot{{
		PluginID:          "my_plugin",
		Name:              "My Plugin",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "disabled",
	}})

	_, err := catalog.SetDesiredState("my_plugin", "disabled")
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("got err=%v, want ErrStateConflict", err)
	}
}
