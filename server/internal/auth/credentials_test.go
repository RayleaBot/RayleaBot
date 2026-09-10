package auth

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCredentialChangePreservesStateOnRejection(t *testing.T) {
	for _, tc := range []struct {
		name, current, next string
		persistenceError    error
		want                error
	}{
		{name: "wrong password", current: "wrong", next: "new-password", want: ErrInvalidCredentials},
		{name: "short password", current: "old-password", next: "short", want: ErrInvalidCredentialInput},
		{name: "persistence failure", current: "old-password", next: "new-password", persistenceError: errors.New("write failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryAuthRepository{updateErr: tc.persistenceError}
			manager := newRepositoryBackedTestManager(t, repo)
			token, claims, err := manager.Bootstrap("admin", "old-password")
			if err != nil {
				t.Fatal(err)
			}
			err = manager.UpdateCredentialsWithContext(context.Background(), claims, tc.current, tc.next, "renamed")
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("unexpected rejection: %v", err)
			}
			if _, err := manager.Validate(token); err != nil {
				t.Fatalf("session lost: %v", err)
			}
			if _, _, err := manager.Login("admin", "old-password"); err != nil {
				t.Fatalf("credentials changed after rejection: %v", err)
			}
		})
	}
}

func TestCredentialChangePersistsAndRevokesAcrossRestart(t *testing.T) {
	for _, identifier := range []string{"", "   ", " renamed "} {
		t.Run("identifier="+identifier, func(t *testing.T) {
			db := filepath.Join(t.TempDir(), "state.db")
			cfg := Config{SessionTTLDays: 1, MaxSessions: 3}
			now := fixedClock(time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
			manager, closeManager := newPersistentManager(t, db, cfg, now, "before")
			token, claims, err := manager.Bootstrap("admin", "old-password")
			if err != nil {
				t.Fatal(err)
			}
			other, _, err := manager.Login("admin", "old-password")
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.UpdateCredentialsWithContext(context.Background(), claims, "old-password", "  新密码保留两端空格  ", identifier); err != nil {
				t.Fatal(err)
			}
			for _, oldToken := range []string{token, other} {
				if _, err := manager.Validate(oldToken); !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("old token still valid: %v", err)
				}
			}
			if err := closeManager(); err != nil {
				t.Fatal(err)
			}
			restored, closeRestored := newPersistentManager(t, db, cfg, now, "after")
			defer func(release func() error) { _ = release() }(closeRestored)
			for _, oldToken := range []string{token, other} {
				if _, err := restored.Validate(oldToken); !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("revoked token restored: %v", err)
				}
			}
			if _, _, err := restored.Login("admin", "old-password"); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("old credentials restored: %v", err)
			}
			want := "admin"
			if identifier == " renamed " {
				want = "renamed"
			}
			if _, _, err := restored.Login(want, "  新密码保留两端空格  "); err != nil {
				t.Fatalf("new credentials unavailable: %v", err)
			}
		})
	}
}

func TestConcurrentCredentialChangesAcceptOnlyOne(t *testing.T) {
	manager := newRepositoryBackedTestManager(t, &memoryAuthRepository{})
	_, claims, err := manager.Bootstrap("admin", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			results <- manager.UpdateCredentialsWithContext(context.Background(), claims, "old-password", "new-password", "")
		}()
	}
	start.Done()
	succeeded := 0
	for range 2 {
		err := <-results
		if err == nil {
			succeeded++
		} else if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("accepted %d changes, want one", succeeded)
	}
}

func TestCredentialChangeRollsBackWhenSessionDeletionFails(t *testing.T) {
	db := filepath.Join(t.TempDir(), "state.db")
	cfg := Config{SessionTTLDays: 1, MaxSessions: 3}
	now := fixedClock(time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	manager, closeManager := newPersistentManager(t, db, cfg, now, "before")
	token, claims, err := manager.Bootstrap("admin", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	repo := manager.repo.(*SQLiteRepository)
	// Fail the second write of the transaction, after the credential UPDATE.
	if _, err := repo.write.Exec(`CREATE TRIGGER reject_session_delete BEFORE DELETE ON admin_sessions BEGIN SELECT RAISE(ABORT, 'fixture deletion failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateCredentialsWithContext(context.Background(), claims, "old-password", "new-password", "renamed"); err == nil {
		t.Fatal("expected transaction failure")
	}
	if _, err := manager.Validate(token); err != nil {
		t.Fatalf("session lost on rollback: %v", err)
	}
	if err := closeManager(); err != nil {
		t.Fatal(err)
	}
	restored, closeRestored := newPersistentManager(t, db, cfg, now, "after")
	defer func(release func() error) { _ = release() }(closeRestored)
	if _, err := restored.Validate(token); err != nil {
		t.Fatalf("persisted session lost on rollback: %v", err)
	}
	if _, _, err := restored.Login("admin", "old-password"); err != nil {
		t.Fatalf("persisted credentials changed on rollback: %v", err)
	}
}
