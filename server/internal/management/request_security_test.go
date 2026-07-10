package management

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
)

func TestValidSetupRequestRejectsCrossSiteAndNonJSONInputs(t *testing.T) {
	config := AuthConfig{
		AllowedHosts:   []string{"127.0.0.1:8080"},
		AllowedOrigins: []string{"http://127.0.0.1:4173"},
	}

	tests := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{name: "invalid host", mutate: func(request *http.Request) { request.Host = "attacker.invalid" }},
		{name: "host port mismatch", mutate: func(request *http.Request) { request.Host = "127.0.0.1:8081" }},
		{name: "invalid origin", mutate: func(request *http.Request) { request.Header.Set("Origin", "https://attacker.invalid") }},
		{name: "cross site fetch", mutate: func(request *http.Request) { request.Header.Set("Sec-Fetch-Site", "cross-site") }},
		{name: "form navigation", mutate: func(request *http.Request) { request.Header.Set("Sec-Fetch-Mode", "navigate") }},
		{name: "plain text", mutate: func(request *http.Request) { request.Header.Set("Content-Type", "text/plain") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validSetupSecurityRequest()
			test.mutate(request)
			if validSetupRequest(request, config) {
				t.Fatal("unsafe setup request was accepted")
			}
		})
	}

	if !validSetupRequest(validSetupSecurityRequest(), config) {
		t.Fatal("same-origin JSON setup request was rejected")
	}
}

func TestOneTimeTokenCannotBeReused(t *testing.T) {
	token := NewOneTimeToken("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if !token.Consume("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA") {
		t.Fatal("first token use was rejected")
	}
	if token.Consume("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA") {
		t.Fatal("consumed setup token was accepted again")
	}
}

func TestCookieAuthRequiresCSRFForStateChanges(t *testing.T) {
	manager := newRequestSecurityAuthManager(t)
	token, claims, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	config := testAuthConfigSource{config: AuthConfig{
		AllowedHosts:   []string{"127.0.0.1:8080"},
		AllowedOrigins: []string{"http://127.0.0.1:4173"},
	}}

	called := false
	handler := RequireAuthWithConfig(manager, config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	withoutCSRF := cookieRequest(http.MethodPost, "/api/system/shutdown", token)
	withoutCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(withoutCSRFResponse, withoutCSRF)
	if withoutCSRFResponse.Code != http.StatusUnauthorized || called {
		t.Fatalf("missing CSRF status=%d called=%v", withoutCSRFResponse.Code, called)
	}

	withCSRF := cookieRequest(http.MethodPost, "/api/system/shutdown", token)
	withCSRF.Header.Set(CSRFHeader, manager.CSRFToken(claims))
	withCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(withCSRFResponse, withCSRF)
	if withCSRFResponse.Code != http.StatusNoContent || !called {
		t.Fatalf("valid CSRF status=%d called=%v", withCSRFResponse.Code, called)
	}
	if withCSRFResponse.Header().Get(CSRFHeader) != manager.CSRFToken(claims) {
		t.Fatal("cookie response did not refresh the in-memory CSRF value")
	}
}

func TestWebSocketAuthRejectsQueryTokensAndUnknownOrigins(t *testing.T) {
	manager := newRequestSecurityAuthManager(t)
	token, _, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	config := testAuthConfigSource{config: AuthConfig{
		AllowedHosts:   []string{"127.0.0.1:8080"},
		AllowedOrigins: []string{"http://127.0.0.1:4173"},
	}}

	called := false
	handler := RequireAuthWithConfig(manager, config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	queryRequest := webSocketSecurityRequest("/ws/events?session_token="+token, "http://127.0.0.1:4173", token)
	queryResponse := httptest.NewRecorder()
	handler.ServeHTTP(queryResponse, queryRequest)
	if queryResponse.Code != http.StatusUnauthorized || called {
		t.Fatal("websocket query token was accepted")
	}

	unknownOriginRequest := webSocketSecurityRequest("/ws/events", "https://attacker.invalid", token)
	unknownOriginResponse := httptest.NewRecorder()
	handler.ServeHTTP(unknownOriginResponse, unknownOriginRequest)
	if unknownOriginResponse.Code != http.StatusUnauthorized || called {
		t.Fatal("unknown websocket origin was accepted")
	}

	validRequest := webSocketSecurityRequest("/ws/events", "http://127.0.0.1:4173", token)
	validResponse := httptest.NewRecorder()
	handler.ServeHTTP(validResponse, validRequest)
	if validResponse.Code != http.StatusNoContent || !called {
		t.Fatalf("valid websocket admission status=%d called=%v", validResponse.Code, called)
	}
}

func validSetupSecurityRequest() *http.Request {
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/setup/admin", nil)
	request.Host = "127.0.0.1:8080"
	request.RemoteAddr = "127.0.0.1:52000"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://127.0.0.1:4173")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("Sec-Fetch-Mode", "cors")
	return request
}

func cookieRequest(method, path, token string) *http.Request {
	request := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, nil)
	request.Host = "127.0.0.1:8080"
	request.Header.Set("Origin", "http://127.0.0.1:4173")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	return request
}

func webSocketSecurityRequest(path, origin, token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+path, nil)
	request.Host = "127.0.0.1:8080"
	request.Header.Set("Origin", origin)
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func newRequestSecurityAuthManager(t *testing.T) *auth.Manager {
	t.Helper()
	manager, err := auth.NewManager(auth.Config{
		SessionTTLDays:         1,
		SessionAbsoluteTTLDays: 30,
		MaxSessions:            3,
	}, auth.WithClock(func() time.Time {
		return time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	}), auth.WithSigningKey([]byte("request-security-test-signing-key")))
	if err != nil {
		t.Fatalf("new auth manager: %v", err)
	}
	return manager
}
