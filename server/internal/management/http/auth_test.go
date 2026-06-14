package managementhttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
)

type testAuthConfigSource struct {
	config AuthConfig
}

func (s testAuthConfigSource) AuthConfig() AuthConfig {
	return s.config
}

func TestAuthHTTPSetupAndLoginResponseShape(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewManager(auth.Config{
		SessionTTLDays: 1,
		SlidingRenewal: false,
		MaxSessions:    3,
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	handlers := NewAuthHandlers(AuthDeps{
		Config: testAuthConfigSource{},
		Auth:   manager,
	})

	setupBody := authHTTPJSON(t, map[string]string{
		"identifier": "admin",
		"secret":     "fixture-only-secret",
	})
	setupResponse := httptest.NewRecorder()
	handlers.HandleSetupAdmin().ServeHTTP(setupResponse, httptest.NewRequest(http.MethodPost, "/api/setup/admin", setupBody))

	assertAuthHTTPSessionTokenResponse(t, setupResponse)

	loginBody := authHTTPJSON(t, map[string]string{
		"identifier": "admin",
		"secret":     "fixture-only-secret",
	})
	loginResponse := httptest.NewRecorder()
	handlers.HandleSessionLogin().ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/session/login", loginBody))

	assertAuthHTTPSessionTokenResponse(t, loginResponse)
}

func authHTTPJSON(t *testing.T, payload any) *bytes.Reader {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auth request: %v", err)
	}
	return bytes.NewReader(body)
}

func assertAuthHTTPSessionTokenResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload) != 1 || payload["session_token"] == "" {
		t.Fatalf("unexpected auth response shape: %#v", payload)
	}
}
