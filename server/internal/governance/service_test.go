package governance

import (
	"context"
	"testing"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestServiceWritesNotifyGovernanceChangedOnce(t *testing.T) {
	t.Parallel()

	blacklist := newStubBlacklistRepo()
	whitelist := newStubWhitelistRepo()
	whitelistState := &stubWhitelistStateRepo{}
	notifications := 0

	service := NewService(Deps{
		CurrentConfig:  func() config.Config { return config.Config{} },
		Plugins:        plugincatalog.New(nil),
		BlacklistRepo:  blacklist,
		WhitelistRepo:  whitelist,
		WhitelistState: whitelistState,
		NotifyChanged: func(summary string) {
			notifications++
			if summary == "" {
				t.Fatal("expected non-empty governance summary")
			}
		},
	})

	if _, err := service.UpsertBlacklistEntry(context.Background(), "user", "1001", "spam"); err != nil {
		t.Fatalf("UpsertBlacklistEntry: %v", err)
	}
	if notifications != 1 {
		t.Fatalf("blacklist upsert notifications = %d, want 1", notifications)
	}

	if _, err := service.UpsertWhitelistEntry(context.Background(), "group", "2001", "approved"); err != nil {
		t.Fatalf("UpsertWhitelistEntry: %v", err)
	}
	if notifications != 2 {
		t.Fatalf("whitelist upsert notifications = %d, want 2", notifications)
	}

	if _, err := service.SetWhitelistEnabled(context.Background(), true); err != nil {
		t.Fatalf("SetWhitelistEnabled: %v", err)
	}
	if notifications != 3 {
		t.Fatalf("whitelist state notifications = %d, want 3", notifications)
	}

	if err := service.DeleteBlacklistEntry(context.Background(), "user", "1001"); err != nil {
		t.Fatalf("DeleteBlacklistEntry: %v", err)
	}
	if notifications != 4 {
		t.Fatalf("blacklist delete notifications = %d, want 4", notifications)
	}

	if err := service.DeleteWhitelistEntry(context.Background(), "group", "2001"); err != nil {
		t.Fatalf("DeleteWhitelistEntry: %v", err)
	}
	if notifications != 5 {
		t.Fatalf("whitelist delete notifications = %d, want 5", notifications)
	}
}
