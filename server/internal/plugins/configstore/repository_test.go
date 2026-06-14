package configstore

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

func TestSQLiteRepositorySeedDefaultsAndReadWrite(t *testing.T) {
	t.Parallel()

	repo, err := NewSQLiteRepository(openTestStore(t))
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	ctx := context.Background()
	pluginID := "weather"

	created, err := repo.SeedDefaults(ctx, pluginID, map[string]any{
		"default_city": "北京",
		"unit":         "celsius",
	})
	if err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if !created {
		t.Fatalf("SeedDefaults created = false, want true")
	}

	values, err := repo.Read(ctx, pluginID, []string{"default_city", "unit", "missing"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if values["default_city"] != "北京" || values["unit"] != "celsius" {
		t.Fatalf("unexpected seeded values: %#v", values)
	}
	if _, ok := values["missing"]; ok {
		t.Fatalf("missing key should not be returned: %#v", values)
	}

	created, err = repo.SeedDefaults(ctx, pluginID, map[string]any{
		"default_city": "上海",
		"unit":         "fahrenheit",
	})
	if err != nil {
		t.Fatalf("SeedDefaults second call: %v", err)
	}
	if created {
		t.Fatalf("SeedDefaults created = true on second call, want false")
	}

	written, err := repo.Write(ctx, pluginID, map[string]any{
		"default_city": "上海",
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
	if values["default_city"] != "上海" || values["unit"] != "fahrenheit" {
		t.Fatalf("unexpected updated values: %#v", values)
	}

	allValues, err := repo.ReadAll(ctx, pluginID)
	if err != nil {
		t.Fatalf("ReadAll after Write: %v", err)
	}
	if allValues["default_city"] != "上海" || allValues["unit"] != "fahrenheit" {
		t.Fatalf("unexpected all values: %#v", allValues)
	}
}
