package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
)

func TestOpaqueSessionStoresOnlyHashAndBindsCSRF(t *testing.T) {
	t.Parallel()
	repository := &memoryAuthRepository{}
	manager, err := NewManager(Config{SessionTTLDays: 1, MaxSessions: 3}, WithRepository(repository))
	if err != nil {
		t.Fatal(err)
	}
	token, claims, err := manager.Issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("session entropy bytes = %d, %v", len(decoded), err)
	}
	digest := sha256.Sum256([]byte(token))
	wantHash := hex.EncodeToString(digest[:])
	if claims.TokenHash != wantHash || len(repository.savedSessions) != 1 || repository.savedSessions[0] != claims {
		t.Fatalf("persisted session does not contain the token hash: %+v", repository.savedSessions)
	}
	if len(manager.sessions) != 1 || manager.sessions[wantHash] != claims {
		t.Fatal("memory session table is not keyed by the token hash")
	}
	if _, err := manager.Validate(claims.TokenHash); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("stored hash accepted as a session token: %v", err)
	}
	csrf := manager.CSRFToken(token)
	if !manager.ValidateCSRF(token, csrf) {
		t.Fatal("session rejected its own CSRF token")
	}
	other, _, err := manager.Issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]string{
		"missing":           "",
		"changed":           replaceLastCharacter(csrf),
		"another session":   manager.CSRFToken(other),
		"stored hash":       manager.CSRFToken(claims.TokenHash),
		"raw session token": token,
	} {
		if manager.ValidateCSRF(token, candidate) {
			t.Errorf("accepted CSRF value from %s", name)
		}
	}
	if err := manager.RevokeWithContext(t.Context(), claims.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Validate(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked token accepted: %v", err)
	}
	if _, err := manager.Validate(other); err != nil {
		t.Fatalf("revoking one session affected another: %v", err)
	}
}
