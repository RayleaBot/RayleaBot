package auth

import (
	"errors"
	"testing"
	"time"
)

func TestIssueAndValidateAcceptsValidToken(t *testing.T) {
	t.Parallel()

	now := fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC))
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, now)

	token, issued, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if claims.SessionID != issued.SessionID {
		t.Fatalf("unexpected session id: got %q want %q", claims.SessionID, issued.SessionID)
	}
	if claims.Subject != "admin" {
		t.Fatalf("unexpected subject: got %q want %q", claims.Subject, "admin")
	}
	if !claims.ExpiresAt.Equal(issued.ExpiresAt) {
		t.Fatalf("unexpected expiry: got %s want %s", claims.ExpiresAt, issued.ExpiresAt)
	}
}

func TestIssueAndValidateAcceptsValidTokenWithSubSecondClock(t *testing.T) {
	t.Parallel()

	now := fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 123456789, time.UTC))
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, now)

	token, issued, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate failed with sub-second clock: %v", err)
	}

	if claims.SessionID != issued.SessionID {
		t.Fatalf("unexpected session id: got %q want %q", claims.SessionID, issued.SessionID)
	}
	if claims.IssuedAt.Nanosecond() != 0 {
		t.Fatalf("expected issued time to be normalized to seconds, got %s", claims.IssuedAt)
	}
}

func TestValidateRejectsInvalidTokens(t *testing.T) {
	t.Parallel()

	now := fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC))
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, now)

	token, _, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	cases := map[string]string{
		"empty":      "",
		"malformed":  "not-a-token",
		"tampered":   token + "corrupted",
		"wrong-sign": replaceLastCharacter(token),
	}

	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := manager.Validate(candidate)
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("expected ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestValidateRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, func() time.Time {
		return current
	})

	token, _, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	current = current.Add(24*time.Hour + time.Second)

	_, err = manager.Validate(token)
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestValidateRenewsExpiryWhenSlidingRenewalEnabled(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: true,
		MaxSessions:    2,
	}, func() time.Time {
		return current
	})

	token, issued, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	current = current.Add(12 * time.Hour)

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	wantExpiry := current.Add(24 * time.Hour)
	if !claims.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("unexpected renewed expiry: got %s want %s", claims.ExpiresAt, wantExpiry)
	}
	if !claims.ExpiresAt.After(issued.ExpiresAt) {
		t.Fatalf("expected renewed expiry to extend beyond original expiry")
	}
}

func TestIssueRecyclesOldestSessionWhenMaxSessionsReached(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    1,
	}, func() time.Time {
		return current
	})

	firstToken, _, err := manager.Issue("admin-a")
	if err != nil {
		t.Fatalf("first Issue failed: %v", err)
	}

	current = current.Add(time.Second)

	secondToken, claims, err := manager.Issue("admin-b")
	if err != nil {
		t.Fatalf("second Issue failed: %v", err)
	}
	if claims.Subject != "admin-b" {
		t.Fatalf("unexpected subject after recycle: got %q want %q", claims.Subject, "admin-b")
	}
	if _, err := manager.Validate(secondToken); err != nil {
		t.Fatalf("expected recycled session token to validate, got %v", err)
	}
	if _, err := manager.Validate(firstToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected oldest token to be invalid after recycle, got %v", err)
	}
}

func TestBootstrapInitializesCredentialSourceAndIssuesToken(t *testing.T) {
	t.Parallel()

	now := fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC))
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, now)

	token, claims, err := manager.Bootstrap("admin", "fixture-only-secret")
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}
	if token == "" {
		t.Fatalf("expected session token to be issued")
	}
	if claims.Subject != "admin" {
		t.Fatalf("unexpected subject: got %q want admin", claims.Subject)
	}
	if !manager.IsBootstrapped() {
		t.Fatalf("expected manager to be bootstrapped")
	}
}

func TestBootstrapRejectsRepeatedInitialization(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    2,
	}, fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)))

	if _, _, err := manager.Bootstrap("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("first Bootstrap failed: %v", err)
	}

	_, _, err := manager.Bootstrap("admin", "fixture-only-secret")
	if !errors.Is(err, ErrBootstrapAlreadyInitialized) {
		t.Fatalf("expected ErrBootstrapAlreadyInitialized, got %v", err)
	}
}

func TestLoginIssuesTokenForBootstrappedCredentials(t *testing.T) {
	t.Parallel()

	now := fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC))
	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    3,
	}, now)

	if _, _, err := manager.Bootstrap("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	token, claims, err := manager.Login("admin", "fixture-only-secret")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Fatalf("expected session token to be issued")
	}
	if claims.Subject != "admin" {
		t.Fatalf("unexpected subject: got %q want admin", claims.Subject)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    3,
	}, fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)))

	if _, _, err := manager.Bootstrap("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	_, _, err := manager.Login("admin", "wrong-secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func newTestManager(t *testing.T, cfg Config, now func() time.Time) *Manager {
	t.Helper()

	sessionCounter := 0
	manager, err := NewManager(
		cfg,
		WithClock(now),
		WithSigningKey([]byte("0123456789abcdef0123456789abcdef")),
		WithSessionIDGenerator(func() (string, error) {
			sessionCounter++
			return "session-" + string(rune('0'+sessionCounter)), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager
}

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time {
		return now
	}
}

func replaceLastCharacter(token string) string {
	if token == "" {
		return token
	}
	if token[len(token)-1] == 'A' {
		return token[:len(token)-1] + "B"
	}
	return token[:len(token)-1] + "A"
}
