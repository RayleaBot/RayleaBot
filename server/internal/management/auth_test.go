package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
)

type testAuthConfigSource struct {
	config AuthConfig
}

func (s testAuthConfigSource) AuthConfig() AuthConfig {
	return s.config
}

func authHTTPJSON(t *testing.T, payload any) *bytes.Reader {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auth request: %v", err)
	}
	return bytes.NewReader(body)
}

func TestAccountCredentialVerificationUsesLoginFailureLimit(t *testing.T) {
	manager, err := auth.NewManager(auth.Config{SessionTTLDays: 1, MaxSessions: 3})
	if err != nil {
		t.Fatal(err)
	}
	_, claims, err := manager.Bootstrap("admin", "fixture-only-secret")
	if err != nil {
		t.Fatal(err)
	}
	tracker := auth.NewLoginFailureTracker(time.Now)
	handlers := NewAuthHandlers(AuthDeps{Auth: manager, LoginFailures: tracker, Config: testAuthConfigSource{config: AuthConfig{LoginFailureLimit: 1, LoginFailureWindow: time.Minute}}})
	request := httptest.NewRequest(http.MethodPut, "/api/account/credentials", authHTTPJSON(t, map[string]string{"current_secret": "wrong", "new_secret": "new-password"}))
	request = request.WithContext(ContextWithClaims(request.Context(), claims))
	response := httptest.NewRecorder()
	handlers.HandleAccountCredentialsUpdate().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("password verification status: %d", response.Code)
	}
	login := httptest.NewRequest(http.MethodPost, "/api/session/login", authHTTPJSON(t, map[string]string{"identifier": "admin", "secret": "fixture-only-secret"}))
	response = httptest.NewRecorder()
	handlers.HandleSessionLogin().ServeHTTP(response, login)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("login limit not shared: %d", response.Code)
	}
}
