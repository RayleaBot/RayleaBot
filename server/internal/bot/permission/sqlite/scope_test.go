package sqlite

import (
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestAccessListNamespacesDoNotShareIDs(t *testing.T) {
	store := openPermissionTestStore(t)
	for _, list := range []string{permission.ListBlacklist, permission.ListWhitelist} {
		t.Run(list, func(t *testing.T) {
			repo := NewAccessListRepository(store.Read, store.Write, list)
			first := chatevent.IdentityScope{Kind: "instance", SourceProtocol: "qqofficial", SourceAdapter: "qq-a", BotID: "bot-a"}
			if err := repo.Add(t.Context(), first, "user", "shared-id", "reviewed"); err != nil {
				t.Fatal(err)
			}
			otherAdapter := first
			otherAdapter.SourceAdapter = "qq-b"
			otherBot := first
			otherBot.BotID = "bot-b"
			otherProtocol := first
			otherProtocol.SourceProtocol = "onebot11"
			for _, scope := range []chatevent.IdentityScope{first, otherAdapter, otherBot, otherProtocol} {
				matched, err := repo.Contains(t.Context(), scope, "user", "shared-id")
				if err != nil || matched != (scope == first) {
					t.Fatalf("scope=%+v matched=%v error=%v", scope, matched, err)
				}
			}
			global := chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}
			if err := repo.Add(t.Context(), global, "user", "shared-id", "all QQ accounts"); err != nil {
				t.Fatal(err)
			}
			if matched, err := repo.Contains(t.Context(), otherProtocol, "user", "shared-id"); err != nil || !matched {
				t.Fatalf("global OneBot rule missing: %v", err)
			}
			if err := repo.Remove(t.Context(), global, "user", "shared-id"); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Get(t.Context(), first, "user", "shared-id"); err != nil {
				t.Fatalf("deleting another scope removed the QQ entry: %v", err)
			}
		})
	}
}

func TestCommandCooldownAndSuperAdminsRespectNamespaces(t *testing.T) {
	limit := config.RateLimit{Count: 1, Window: time.Hour}
	checker := permission.NewChecker(permission.CheckerConfig{SuperAdmins: []string{"admin-id"}}, nil, nil, nil, permission.NewCooldownTracker(limit, limit))
	first := chatevent.IdentityScope{Kind: "instance", SourceProtocol: "qqofficial", SourceAdapter: "qq", BotID: "bot-a"}
	command := &permission.CommandInfo{Permission: "everyone"}
	for _, scope := range []chatevent.IdentityScope{
		first,
		{Kind: "instance", SourceProtocol: "qqofficial", SourceAdapter: "qq", BotID: "bot-b"},
		{Kind: "instance", SourceProtocol: "qqofficial", SourceAdapter: "another", BotID: "bot-a"},
		{Kind: "instance", SourceProtocol: "onebot11", SourceAdapter: "qq", BotID: "bot-a"},
	} {
		if result := checker.Check(t.Context(), scope, "same-user", "member", "same-group", command); !result.Allowed {
			t.Fatalf("namespace reused another cooldown: %+v", result)
		}
	}
	if result := checker.Check(t.Context(), first, "same-user", "member", "same-group", command); result.ErrorCode != "platform.user_rate_limited" {
		t.Fatal(result)
	}
	if result := checker.Check(t.Context(), first, "admin-id", "member", "", &permission.CommandInfo{Permission: "super_admin"}); result.Allowed {
		t.Fatal("QQ openid inherited OneBot super-admin privileges")
	}
}
