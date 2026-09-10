package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestConfigSQLiteRepositoryReadAndWrite(t *testing.T) {
	t.Parallel()

	repo, err := NewConfigSQLiteRepository(openTestStore(t))
	if err != nil {
		t.Fatalf("NewConfigSQLiteRepository: %v", err)
	}

	ctx := context.Background()
	pluginID := "weather"

	if _, err := repo.Write(ctx, pluginID, map[string]any{"default_city": "Beijing", "unit": "celsius"}); err != nil {
		t.Fatal(err)
	}
	values, err := repo.Read(ctx, pluginID, []string{"default_city", "unit", "missing"})
	if err != nil || values["default_city"] != "Beijing" || values["unit"] != "celsius" {
		t.Fatalf("read stored settings: %#v %v", values, err)
	}
	if _, exists := values["missing"]; exists {
		t.Fatal("missing key unexpectedly returned")
	}

	written, err := repo.Write(ctx, pluginID, map[string]any{
		"default_city": "Shanghai",
		"unit":         "fahrenheit",
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(written) != 2 || written[0] != "default_city" || written[1] != "unit" {
		t.Fatalf("unexpected written keys: %#v", written)
	}

	values, err = repo.Read(ctx, pluginID, []string{"default_city", "unit"})
	if err != nil {
		t.Fatalf("Read after Write: %v", err)
	}
	if values["default_city"] != "Shanghai" || values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected updated values: %#v", values)
	}

	allValues, err := repo.ReadAll(ctx, pluginID)
	if err != nil {
		t.Fatalf("ReadAll after Write: %v", err)
	}
	if allValues["default_city"] != "Shanghai" || allValues["unit"] != "fahrenheit" {
		t.Fatalf("unexpected all values: %#v", allValues)
	}
}

func TestMergeValuesReturnsIndependentEffectiveSnapshot(t *testing.T) {
	t.Parallel()

	defaults := map[string]any{
		"enabled": true,
		"nested":  map[string]any{"source": "default"},
		"items":   []any{"default"},
	}
	persisted := map[string]any{
		"enabled": false,
		"nested":  map[string]any{"source": "persisted"},
	}

	merged := MergeValues(defaults, persisted)
	if merged["enabled"] != false {
		t.Fatalf("persisted value did not override default: %#v", merged)
	}
	if merged["nested"].(map[string]any)["source"] != "persisted" {
		t.Fatalf("persisted nested value did not override default: %#v", merged)
	}
	if merged["items"].([]any)[0] != "default" {
		t.Fatalf("default-only value is missing: %#v", merged)
	}

	merged["nested"].(map[string]any)["source"] = "changed"
	merged["items"].([]any)[0] = "changed"
	if persisted["nested"].(map[string]any)["source"] != "persisted" {
		t.Fatalf("MergeValues mutated persisted values: %#v", persisted)
	}
	if defaults["items"].([]any)[0] != "default" {
		t.Fatalf("MergeValues mutated default values: %#v", defaults)
	}
}

func TestConfigWriteNoopAndExactKeys(t *testing.T) {
	t.Parallel()
	repo, err := NewConfigSQLiteRepository(openTestStore(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	values := map[string]any{" spaced ": "value", "nested": map[string]any{"a": 1, "b": true}}
	first, err := repo.Write(ctx, "weather", values)
	if err != nil || len(first) != 2 {
		t.Fatalf("first write: %v %v", first, err)
	}
	again, err := repo.Write(ctx, "weather", map[string]any{" spaced ": "value", "nested": map[string]any{"b": true, "a": float64(1)}})
	if err != nil || len(again) != 0 {
		t.Fatalf("same JSON reported changes: %v %v", again, err)
	}
	read, err := repo.Read(ctx, "weather", []string{" spaced "})
	if err != nil || read[" spaced "] != "value" {
		t.Fatalf("exact key lost: %#v %v", read, err)
	}
	if _, err := repo.Write(ctx, "weather", map[string]any{"": "invalid", "new": true}); err == nil {
		t.Fatal("empty key accepted")
	}
	all, err := repo.ReadAll(ctx, "weather")
	if err != nil || len(all) != 2 {
		t.Fatalf("invalid write partially committed: %#v %v", all, err)
	}
}
