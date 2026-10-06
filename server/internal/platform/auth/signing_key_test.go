package auth

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSigningKeyRotationSurvivesHydrationAndRevokesSessions(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	oldKey, newKey := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	manager, err := NewManagerWithContext(t.Context(), Config{SessionTTLDays: 1, SessionAbsoluteTTLDays: 30, MaxSessions: 3}, WithRepository(repo), WithSigningKey(oldKey))
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.Bootstrap("admin", "fixture-only-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ReconcileSigningKey(t.Context(), oldKey); err != nil {
		t.Fatal(err)
	}
	sessions, err := repo.LoadSessions(t.Context())
	if err != nil || len(sessions) != 1 {
		t.Fatalf("unchanged key revoked sessions: %v %v", len(sessions), err)
	}
	if err := repo.ReconcileSigningKey(t.Context(), newKey); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewManagerWithContext(t.Context(), Config{SessionTTLDays: 1, SessionAbsoluteTTLDays: 30, MaxSessions: 3}, WithRepository(repo), WithSigningKey(newKey))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restarted.signingKey, newKey) {
		t.Fatal("hydration restored the old key")
	}
	if _, err := restarted.Validate(token); err == nil {
		t.Fatal("old session survived rotation")
	}
	newToken, _, err := restarted.Login("admin", "fixture-only-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Validate(newToken); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write.ExecContext(t.Context(), "INSERT INTO secret_store VALUES (?, ?, ?, ?)", sessionSigningKeySecret, newKey, "2026-10-06T00:00:00Z", "2026-10-06T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := resetCredentials(t.Context(), store.Write); err != nil {
		t.Fatal(err)
	}
	var keys int
	if err := store.Read.QueryRowContext(t.Context(), "SELECT count(*) FROM secret_store WHERE key = ?", sessionSigningKeySecret).Scan(&keys); err != nil || keys != 0 {
		t.Fatalf("reset retained the signing key: %d %v", keys, err)
	}
}

func TestSigningKeyRotationRollsBackIfSessionRevocationFails(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{1}, 32)
	manager, err := NewManagerWithContext(t.Context(), Config{SessionTTLDays: 1, SessionAbsoluteTTLDays: 30, MaxSessions: 3}, WithRepository(repo), WithSigningKey(key))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Bootstrap("admin", "fixture-only-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write.Exec("CREATE TRIGGER refuse_rotation BEFORE DELETE ON admin_sessions BEGIN SELECT RAISE(ABORT, 'fixture failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReconcileSigningKey(t.Context(), bytes.Repeat([]byte{2}, 32)); err == nil {
		t.Fatal("rotation succeeded")
	}
	state, err := repo.LoadBootstrap(t.Context())
	if err != nil || !bytes.Equal(state.SigningKey, key) {
		t.Fatalf("rotation did not roll back: %v", err)
	}
}
