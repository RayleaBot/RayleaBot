package permission

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"testing"
	"time"
)

// failingEntryRepo fails every Contains lookup for one entry type and defers
// the rest to the wrapped repository.
type failingEntryRepo struct {
	EntryRepository
	entryType string
}

func (r failingEntryRepo) Contains(ctx context.Context, scope chatevent.IdentityScope, kind, id string) (bool, error) {
	if kind == r.entryType {
		return false, context.DeadlineExceeded
	}
	return r.EntryRepository.Contains(ctx, scope, kind, id)
}

type failingWhitelistState struct{ WhitelistStateRepository }

func (failingWhitelistState) Enabled(context.Context) (bool, error) {
	return false, context.DeadlineExceeded
}

func TestGovernanceReadFailuresDenyWithoutConsumingCooldown(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"whitelist-state", "whitelist-user", "whitelist-group", "blacklist-user", "blacklist-group"} {
		t.Run(stage, func(t *testing.T) {
			var blacklist EntryRepository = newStubBlacklistRepo()
			var whitelist EntryRepository = newStubWhitelistRepo()
			var state WhitelistStateRepository = &stubWhitelistStateRepo{enabled: true}
			switch stage {
			case "whitelist-state":
				state = failingWhitelistState{state}
			case "whitelist-user":
				whitelist = failingEntryRepo{whitelist, "user"}
			case "whitelist-group":
				whitelist = failingEntryRepo{whitelist, "group"}
			case "blacklist-user":
				state = &stubWhitelistStateRepo{}
				blacklist = failingEntryRepo{blacklist, "user"}
			case "blacklist-group":
				state = &stubWhitelistStateRepo{}
				blacklist = failingEntryRepo{blacklist, "group"}
			}
			cooldown := NewCooldownTracker(config.RateLimit{Count: 1, Window: time.Minute}, config.RateLimit{Count: 1, Window: time.Minute})
			checker := NewChecker(CheckerConfig{}, whitelist, state, blacklist, cooldown)
			verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user", "member", "group", &CommandInfo{Permission: "everyone"})
			if verdict.Allowed || verdict.ErrorCode != "permission.unavailable" || !errors.Is(verdict.Err, context.DeadlineExceeded) {
				t.Fatalf("unexpected failure verdict: %#v", verdict)
			}
			recovered := NewChecker(CheckerConfig{}, newStubWhitelistRepo(), &stubWhitelistStateRepo{}, newStubBlacklistRepo(), cooldown)
			if verdict := recovered.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user", "member", "group", &CommandInfo{Permission: "everyone"}); !verdict.Allowed {
				t.Fatalf("failed admission consumed cooldown: %#v", verdict)
			}
		})
	}
}

type stubBlacklistRepo struct {
	blocked map[string]map[string]bool
}

func newStubBlacklistRepo() *stubBlacklistRepo {
	return &stubBlacklistRepo{blocked: make(map[string]map[string]bool)}
}

func (s *stubBlacklistRepo) block(entryType, targetID string) {
	if s.blocked[entryType] == nil {
		s.blocked[entryType] = make(map[string]bool)
	}
	s.blocked[entryType][targetID] = true
}

func (s *stubBlacklistRepo) Contains(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	if m, ok := s.blocked[entryType]; ok {
		return m[targetID], nil
	}
	return false, nil
}

func (s *stubBlacklistRepo) Get(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (Entry, error) {
	if blocked, _ := s.Contains(context.Background(), scope, entryType, targetID); blocked {
		return Entry{EntryType: entryType, TargetID: targetID, Reason: "blocked", CreatedAt: "2026-04-19T00:00:00Z"}, nil
	}
	return Entry{}, ErrGovernanceEntryNotFound
}

func (s *stubBlacklistRepo) Add(_ context.Context, scope chatevent.IdentityScope, entryType, targetID, _ string) error {
	s.block(entryType, targetID)
	return nil
}

func (s *stubBlacklistRepo) Remove(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	if m, ok := s.blocked[entryType]; ok {
		delete(m, targetID)
	}
	return nil
}

func (s *stubBlacklistRepo) List(_ context.Context, _ string) ([]Entry, error) {
	return nil, nil
}

type stubWhitelistRepo struct {
	allowed map[string]map[string]bool
}

func newStubWhitelistRepo() *stubWhitelistRepo {
	return &stubWhitelistRepo{allowed: make(map[string]map[string]bool)}
}

func (s *stubWhitelistRepo) allow(entryType, targetID string) {
	if s.allowed[entryType] == nil {
		s.allowed[entryType] = make(map[string]bool)
	}
	s.allowed[entryType][targetID] = true
}

func (s *stubWhitelistRepo) Contains(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	if m, ok := s.allowed[entryType]; ok {
		return m[targetID], nil
	}
	return false, nil
}

func (s *stubWhitelistRepo) Get(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (Entry, error) {
	if allowed, _ := s.Contains(context.Background(), scope, entryType, targetID); allowed {
		return Entry{EntryType: entryType, TargetID: targetID, Reason: "allowed", CreatedAt: "2026-04-19T00:00:00Z"}, nil
	}
	return Entry{}, ErrGovernanceEntryNotFound
}

func (s *stubWhitelistRepo) Add(_ context.Context, scope chatevent.IdentityScope, entryType, targetID, _ string) error {
	s.allow(entryType, targetID)
	return nil
}

func (s *stubWhitelistRepo) Remove(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	if m, ok := s.allowed[entryType]; ok {
		delete(m, targetID)
	}
	return nil
}

func (s *stubWhitelistRepo) List(_ context.Context, _ string) ([]Entry, error) {
	return nil, nil
}

type stubWhitelistStateRepo struct {
	enabled bool
}

func (s *stubWhitelistStateRepo) Enabled(context.Context) (bool, error) {
	return s.enabled, nil
}

func (s *stubWhitelistStateRepo) SetEnabled(_ context.Context, enabled bool) error {
	s.enabled = enabled
	return nil
}

func TestSuperAdminBypassesAllChecks(t *testing.T) {
	t.Parallel()

	blacklistRepo := newStubBlacklistRepo()
	blacklistRepo.block("user", "admin1")

	checker := NewChecker(
		CheckerConfig{SuperAdmins: []string{"admin1"}},
		newStubWhitelistRepo(),
		&stubWhitelistStateRepo{enabled: true},
		blacklistRepo,
		NewCooldownTracker(
			config.RateLimit{Count: 1, Window: time.Minute},
			config.RateLimit{Count: 1, Window: time.Minute},
		),
	)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "admin1", "member", "group1", &CommandInfo{Permission: "super_admin"})
	if !verdict.Allowed {
		t.Fatalf("super admin should bypass all checks, got denied: %s", verdict.Reason)
	}
}

func TestWhitelistDisabledKeepsExistingBehavior(t *testing.T) {
	t.Parallel()

	checker := NewChecker(
		CheckerConfig{},
		newStubWhitelistRepo(),
		&stubWhitelistStateRepo{enabled: false},
		nil,
		nil,
	)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user1", "member", "", &CommandInfo{Permission: "everyone"})
	if !verdict.Allowed {
		t.Fatalf("whitelist disabled should keep old behavior, got denied: %#v", verdict)
	}
}

func TestWhitelistEnabledAllowsPrivateMessageWhenUserMatches(t *testing.T) {
	t.Parallel()

	whitelistRepo := newStubWhitelistRepo()
	whitelistRepo.allow("user", "10001")
	checker := NewChecker(CheckerConfig{}, whitelistRepo, &stubWhitelistStateRepo{enabled: true}, nil, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "", &CommandInfo{Permission: "everyone"})
	if !verdict.Allowed {
		t.Fatalf("private whitelisted user should be allowed, got %#v", verdict)
	}
}

func TestWhitelistEnabledAllowsGroupMessageWhenUserMatches(t *testing.T) {
	t.Parallel()

	whitelistRepo := newStubWhitelistRepo()
	whitelistRepo.allow("user", "10001")
	checker := NewChecker(CheckerConfig{}, whitelistRepo, &stubWhitelistStateRepo{enabled: true}, nil, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "20001", &CommandInfo{Permission: "everyone"})
	if !verdict.Allowed {
		t.Fatalf("group message should pass when user matches whitelist, got %#v", verdict)
	}
}

func TestWhitelistEnabledAllowsGroupMessageWhenGroupMatches(t *testing.T) {
	t.Parallel()

	whitelistRepo := newStubWhitelistRepo()
	whitelistRepo.allow("group", "20001")
	checker := NewChecker(CheckerConfig{}, whitelistRepo, &stubWhitelistStateRepo{enabled: true}, nil, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "20001", &CommandInfo{Permission: "everyone"})
	if !verdict.Allowed {
		t.Fatalf("group message should pass when group matches whitelist, got %#v", verdict)
	}
}

func TestWhitelistTakesPriorityOverBlacklist(t *testing.T) {
	t.Parallel()

	whitelistRepo := newStubWhitelistRepo()
	whitelistRepo.allow("user", "10001")
	blacklistRepo := newStubBlacklistRepo()
	blacklistRepo.block("user", "10001")

	checker := NewChecker(CheckerConfig{}, whitelistRepo, &stubWhitelistStateRepo{enabled: true}, blacklistRepo, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "", &CommandInfo{Permission: "everyone"})
	if !verdict.Allowed {
		t.Fatalf("whitelist should take priority over blacklist, got %#v", verdict)
	}
}

func TestWhitelistDoesNotBypassPermissionChecks(t *testing.T) {
	t.Parallel()

	whitelistRepo := newStubWhitelistRepo()
	whitelistRepo.allow("user", "10001")
	checker := NewChecker(CheckerConfig{}, whitelistRepo, &stubWhitelistStateRepo{enabled: true}, nil, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "", &CommandInfo{Permission: "super_admin"})
	if verdict.Allowed {
		t.Fatal("whitelist should not bypass permission level checks")
	}
	if verdict.ErrorCode != "permission.denied" {
		t.Fatalf("unexpected error code: got %q want %q", verdict.ErrorCode, "permission.denied")
	}
}

func TestWhitelistDoesNotBypassCooldown(t *testing.T) {
	t.Parallel()

	whitelistRepo := newStubWhitelistRepo()
	whitelistRepo.allow("user", "10001")
	cooldown := NewCooldownTracker(
		config.RateLimit{Count: 1, Window: time.Minute},
		config.RateLimit{Count: 1, Window: time.Minute},
	)
	checker := NewChecker(CheckerConfig{}, whitelistRepo, &stubWhitelistStateRepo{enabled: true}, nil, cooldown)
	command := &CommandInfo{Permission: "everyone"}

	first := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "", command)
	if !first.Allowed {
		t.Fatalf("first whitelisted command should be allowed, got %#v", first)
	}

	second := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "", command)
	if second.Allowed {
		t.Fatal("whitelist should not bypass cooldown")
	}
	if second.ErrorCode != "platform.user_rate_limited" {
		t.Fatalf("unexpected error code: got %q want %q", second.ErrorCode, "platform.user_rate_limited")
	}
}

func TestEmptyEnabledWhitelistBlocksAllNonSuperAdmins(t *testing.T) {
	t.Parallel()

	checker := NewChecker(CheckerConfig{}, newStubWhitelistRepo(), &stubWhitelistStateRepo{enabled: true}, nil, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "10001", "member", "", &CommandInfo{Permission: "everyone"})
	if verdict.Allowed {
		t.Fatal("enabled empty whitelist should block non-super-admin command dispatch")
	}
	if verdict.ErrorCode != "permission.not_whitelisted" {
		t.Fatalf("unexpected error code: got %q want %q", verdict.ErrorCode, "permission.not_whitelisted")
	}
}

func TestBlacklistedUserDeniedWhenWhitelistDoesNotApply(t *testing.T) {
	t.Parallel()

	blacklistRepo := newStubBlacklistRepo()
	blacklistRepo.block("user", "baduser")
	checker := NewChecker(CheckerConfig{}, nil, nil, blacklistRepo, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "baduser", "member", "", nil)
	if verdict.Allowed {
		t.Fatal("blacklisted user should be denied")
	}
	if verdict.ErrorCode != "permission.blacklisted" {
		t.Fatalf("unexpected error code: got %q want %q", verdict.ErrorCode, "permission.blacklisted")
	}
}

func TestBlacklistedGroupDeniedWhenWhitelistDoesNotApply(t *testing.T) {
	t.Parallel()

	blacklistRepo := newStubBlacklistRepo()
	blacklistRepo.block("group", "badgroup")
	checker := NewChecker(CheckerConfig{}, nil, nil, blacklistRepo, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "normaluser", "member", "badgroup", nil)
	if verdict.Allowed {
		t.Fatal("blacklisted group should be denied")
	}
	if verdict.ErrorCode != "permission.blacklisted" {
		t.Fatalf("unexpected error code: got %q want %q", verdict.ErrorCode, "permission.blacklisted")
	}
}

func TestSuperAdminPermissionDeniedForMember(t *testing.T) {
	t.Parallel()

	checker := NewChecker(CheckerConfig{}, nil, nil, nil, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user1", "member", "group1", &CommandInfo{Permission: "super_admin"})
	if verdict.Allowed {
		t.Fatal("member should not be allowed super_admin commands")
	}
	if verdict.ErrorCode != "permission.denied" {
		t.Fatalf("unexpected error code: got %q want %q", verdict.ErrorCode, "permission.denied")
	}
}

func TestGroupAdminPermissionAllowedForAdmin(t *testing.T) {
	t.Parallel()

	checker := NewChecker(CheckerConfig{}, nil, nil, nil, nil)

	verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user1", "admin", "group1", &CommandInfo{Permission: "group_admin"})
	if !verdict.Allowed {
		t.Fatalf("admin role should satisfy group_admin permission, got denied: %s", verdict.Reason)
	}
}

func TestCooldownTriggered(t *testing.T) {
	t.Parallel()

	cooldown := NewCooldownTracker(
		config.RateLimit{Count: 2, Window: time.Minute},
		config.RateLimit{Count: 2, Window: time.Minute},
	)
	checker := NewChecker(CheckerConfig{}, nil, nil, nil, cooldown)
	command := &CommandInfo{Permission: "everyone"}

	first := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user1", "member", "", command)
	if !first.Allowed {
		t.Fatal("first call should be allowed")
	}
	second := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user1", "member", "", command)
	if !second.Allowed {
		t.Fatal("second call should be allowed")
	}

	third := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user1", "member", "", command)
	if third.Allowed {
		t.Fatal("third call should be rate limited")
	}
	if third.ErrorCode != "platform.user_rate_limited" {
		t.Fatalf("unexpected error code: got %q want %q", third.ErrorCode, "platform.user_rate_limited")
	}
}

func TestPrivateMessageSkipsGroupCooldown(t *testing.T) {
	t.Parallel()

	// The group window admits one command; a private command must never
	// consume it, so a second private command is still allowed.
	cooldown := NewCooldownTracker(
		config.RateLimit{Count: 100, Window: time.Minute},
		config.RateLimit{Count: 1, Window: time.Minute},
	)
	checker := NewChecker(CheckerConfig{}, nil, nil, nil, cooldown)
	command := &CommandInfo{Permission: "everyone"}

	for attempt := 1; attempt <= 2; attempt++ {
		verdict := checker.Check(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user1", "member", "", command)
		if !verdict.Allowed {
			t.Fatalf("private command %d consumed the group cooldown: %#v", attempt, verdict)
		}
	}
}
