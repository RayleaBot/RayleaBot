package plugins

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSQLiteRepositoryLoadGrantsFiltersExpiredEntries(t *testing.T) {
	t.Parallel()

	repo := openPluginRepository(t)
	now := time.Now().UTC()
	future := now.Add(2 * time.Hour)
	past := now.Add(-2 * time.Hour)

	for _, grant := range []PluginGrant{
		{
			PluginID:   "weather",
			Capability: "http.request",
			ScopeJSON:  `{"http_hosts":["api.example.com"]}`,
			GrantedAt:  now,
		},
		{
			PluginID:   "weather",
			Capability: "logger.write",
			ScopeJSON:  `{"http_hosts":["api.example.com"]}`,
			GrantedAt:  now,
			ExpiresAt:  &future,
		},
		{
			PluginID:   "weather",
			Capability: "storage.file",
			ScopeJSON:  `{"storage_roots":["plugin_data"]}`,
			GrantedAt:  now,
			ExpiresAt:  &past,
		},
	} {
		if err := repo.SaveGrant(context.Background(), grant); err != nil {
			t.Fatalf("SaveGrant(%s): %v", grant.Capability, err)
		}
	}

	grants, err := repo.LoadGrants(context.Background(), "weather")
	if err != nil {
		t.Fatalf("LoadGrants returned error: %v", err)
	}
	if len(grants) != 2 {
		t.Fatalf("len(grants) = %d, want 2", len(grants))
	}
	if grants[0].Capability != "http.request" || grants[0].ExpiresAt != nil {
		t.Fatalf("unexpected first grant: %#v", grants[0])
	}
	if grants[1].Capability != "logger.write" || grants[1].ExpiresAt == nil {
		t.Fatalf("unexpected second grant: %#v", grants[1])
	}
}

func TestSQLiteRepositoryLoadAllGrantsFiltersExpiredEntries(t *testing.T) {
	t.Parallel()

	repo := openPluginRepository(t)
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	for _, grant := range []PluginGrant{
		{
			PluginID:   "weather",
			Capability: "http.request",
			ScopeJSON:  `{}`,
			GrantedAt:  now,
		},
		{
			PluginID:   "weather",
			Capability: "logger.write",
			ScopeJSON:  `{}`,
			GrantedAt:  now,
			ExpiresAt:  &future,
		},
		{
			PluginID:   "clock",
			Capability: "event.subscribe",
			ScopeJSON:  `{}`,
			GrantedAt:  now,
			ExpiresAt:  &past,
		},
	} {
		if err := repo.SaveGrant(context.Background(), grant); err != nil {
			t.Fatalf("SaveGrant(%s/%s): %v", grant.PluginID, grant.Capability, err)
		}
	}

	all, err := repo.LoadAllGrants(context.Background())
	if err != nil {
		t.Fatalf("LoadAllGrants returned error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len(all) = %d, want 1", len(all))
	}
	if got := all["weather"]; len(got) != 2 {
		t.Fatalf("len(all[weather]) = %d, want 2", len(got))
	}
	if _, ok := all["clock"]; ok {
		t.Fatalf("expired-only plugin should not appear in active grants: %#v", all)
	}
}

func openPluginRepository(t *testing.T) *SQLiteRepository {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("close store: %v", closeErr)
		}
	})

	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository failed: %v", err)
	}
	return repo
}
