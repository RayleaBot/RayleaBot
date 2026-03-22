package permission

import (
	"context"
	"testing"
	"time"
)

// stubBlacklistRepo is a minimal in-memory BlacklistRepository for testing.
type stubBlacklistRepo struct {
	blocked map[string]map[string]bool // entryType -> targetID -> blocked
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

func (s *stubBlacklistRepo) IsBlacklisted(_ context.Context, entryType, targetID string) (bool, error) {
	if m, ok := s.blocked[entryType]; ok {
		return m[targetID], nil
	}
	return false, nil
}

func (s *stubBlacklistRepo) Add(_ context.Context, entryType, targetID, _ string) error {
	s.block(entryType, targetID)
	return nil
}

func (s *stubBlacklistRepo) Remove(_ context.Context, entryType, targetID string) error {
	if m, ok := s.blocked[entryType]; ok {
		delete(m, targetID)
	}
	return nil
}

func (s *stubBlacklistRepo) List(_ context.Context, _ string) ([]BlacklistEntry, error) {
	return nil, nil
}

func TestSuperAdminBypassesAllChecks(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.block("user", "admin1")

	checker := NewChecker(
		CheckerConfig{SuperAdmins: []string{"admin1"}},
		repo,
		NewCooldownTracker(
			RateLimit{Count: 1, Window: time.Minute},
			RateLimit{Count: 1, Window: time.Minute},
		),
	)

	v := checker.Check(context.Background(), "admin1", "member", "group1", &CommandInfo{Permission: "super_admin"})
	if !v.Allowed {
		t.Fatalf("super admin should bypass all checks, got denied: %s", v.Reason)
	}
}

func TestBlacklistedUserDenied(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.block("user", "baduser")

	checker := NewChecker(CheckerConfig{}, repo, nil)

	v := checker.Check(context.Background(), "baduser", "member", "", nil)
	if v.Allowed {
		t.Fatal("blacklisted user should be denied")
	}
	if v.ErrorCode != "permission.blacklisted" {
		t.Fatalf("unexpected error code: got %q want %q", v.ErrorCode, "permission.blacklisted")
	}
}

func TestBlacklistedGroupDenied(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.block("group", "badgroup")

	checker := NewChecker(CheckerConfig{}, repo, nil)

	v := checker.Check(context.Background(), "normaluser", "member", "badgroup", nil)
	if v.Allowed {
		t.Fatal("blacklisted group should be denied")
	}
	if v.ErrorCode != "permission.blacklisted" {
		t.Fatalf("unexpected error code: got %q want %q", v.ErrorCode, "permission.blacklisted")
	}
}

func TestSuperAdminPermissionDeniedForMember(t *testing.T) {
	t.Parallel()

	checker := NewChecker(CheckerConfig{}, nil, nil)

	v := checker.Check(context.Background(), "user1", "member", "group1", &CommandInfo{Permission: "super_admin"})
	if v.Allowed {
		t.Fatal("member should not be allowed super_admin commands")
	}
	if v.ErrorCode != "permission.denied" {
		t.Fatalf("unexpected error code: got %q want %q", v.ErrorCode, "permission.denied")
	}
}

func TestGroupAdminPermissionAllowedForAdmin(t *testing.T) {
	t.Parallel()

	checker := NewChecker(CheckerConfig{}, nil, nil)

	v := checker.Check(context.Background(), "user1", "admin", "group1", &CommandInfo{Permission: "group_admin"})
	if !v.Allowed {
		t.Fatalf("admin role should satisfy group_admin permission, got denied: %s", v.Reason)
	}
}

func TestCooldownTriggered(t *testing.T) {
	t.Parallel()

	cooldown := NewCooldownTracker(
		RateLimit{Count: 2, Window: time.Minute},
		RateLimit{Count: 2, Window: time.Minute},
	)
	checker := NewChecker(CheckerConfig{}, nil, cooldown)
	ctx := context.Background()
	cmd := &CommandInfo{Permission: "everyone"}

	// First two calls should pass.
	v1 := checker.Check(ctx, "user1", "member", "", cmd)
	if !v1.Allowed {
		t.Fatal("first call should be allowed")
	}
	v2 := checker.Check(ctx, "user1", "member", "", cmd)
	if !v2.Allowed {
		t.Fatal("second call should be allowed")
	}

	// Third call should be rate limited.
	v3 := checker.Check(ctx, "user1", "member", "", cmd)
	if v3.Allowed {
		t.Fatal("third call should be rate limited")
	}
	if v3.ErrorCode != "platform.user_rate_limited" {
		t.Fatalf("unexpected error code: got %q want %q", v3.ErrorCode, "platform.user_rate_limited")
	}
}

func TestNilCheckerAllowsEverything(t *testing.T) {
	t.Parallel()

	var checker *Checker
	v := checker.Check(context.Background(), "anyone", "member", "group1", &CommandInfo{Permission: "super_admin"})
	if !v.Allowed {
		t.Fatal("nil checker should allow everything")
	}
}

func TestPrivateMessageSkipsGroupChecks(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.block("group", "group1")

	cooldown := NewCooldownTracker(
		RateLimit{Count: 100, Window: time.Minute},
		RateLimit{Count: 1, Window: time.Minute}, // Very strict group limit.
	)
	checker := NewChecker(CheckerConfig{}, repo, cooldown)

	// Private message (empty groupID) should not trigger group blacklist or group cooldown.
	v := checker.Check(context.Background(), "user1", "member", "", &CommandInfo{Permission: "everyone"})
	if !v.Allowed {
		t.Fatalf("private message should skip group checks, got denied: %s", v.Reason)
	}
}
