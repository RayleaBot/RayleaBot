package app

import (
	"testing"
	"time"
)

func TestLauncherTokenStoreIssuesSingleUseShortLivedTokens(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	store := newLauncherTokenStore(func() time.Time { return now }, 5*time.Minute)

	token, err := store.Issue()
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty launcher token")
	}

	if !store.Consume(token) {
		t.Fatal("expected first consume to succeed")
	}
	if store.Consume(token) {
		t.Fatal("expected second consume to fail for single-use token")
	}
}

func TestLauncherTokenStoreRejectsExpiredTokens(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	store := newLauncherTokenStore(func() time.Time { return now }, 5*time.Minute)

	token, err := store.Issue()
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	now = now.Add(6 * time.Minute)
	if store.Consume(token) {
		t.Fatal("expected expired token consume to fail")
	}
}
