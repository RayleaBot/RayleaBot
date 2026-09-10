package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestResetCredentialsRollsBackSessionRemovalWhenBootstrapDeleteFails(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, statement := range []string{
		`INSERT INTO auth_bootstrap_state VALUES (1, 'admin', X'00', X'00', '2026-09-10T00:00:00Z')`,
		`INSERT INTO admin_sessions VALUES ('session', 'admin', '2026-09-10T00:00:00Z', '2026-09-11T00:00:00Z')`,
		`CREATE TRIGGER reject_reset BEFORE DELETE ON auth_bootstrap_state BEGIN SELECT RAISE(FAIL, 'injected reset failure'); END`,
	} {
		if _, err := store.Write.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := resetCredentials(t.Context(), store.Write); err == nil {
		t.Fatal("reset unexpectedly succeeded")
	}
	repository, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := repository.LoadBootstrap(t.Context())
	if err != nil || bootstrap == nil || bootstrap.Identifier != "admin" {
		t.Fatalf("credentials changed: %#v %v", bootstrap, err)
	}
	sessions, err := repository.LoadSessions(t.Context())
	if err != nil || len(sessions) != 1 || sessions[0].SessionID != "session" {
		t.Fatalf("sessions were lost after failed reset: %#v %v", sessions, err)
	}
}

func TestResetStoredCredentialsDoesNotReplaceMalformedDatabase(t *testing.T) {
	database := filepath.Join(t.TempDir(), "state.db")
	if err := os.WriteFile(database, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ResetStoredCredentials(t.Context(), database); err == nil {
		t.Fatal("malformed database was accepted")
	}
	if got, err := os.ReadFile(database); err != nil || string(got) != "not a database" {
		t.Fatalf("malformed database was replaced: %q %v", got, err)
	}
}
