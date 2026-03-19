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

func TestIssueRejectsWhenMaxSessionsReached(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t, Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    1,
	}, fixedClock(time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)))

	if _, _, err := manager.Issue("admin-a"); err != nil {
		t.Fatalf("first Issue failed: %v", err)
	}

	_, _, err := manager.Issue("admin-b")
	if !errors.Is(err, ErrSessionLimitReached) {
		t.Fatalf("expected ErrSessionLimitReached, got %v", err)
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
