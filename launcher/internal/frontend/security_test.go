package frontend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveDevServerAcceptsOnlyLoopbackDevelopmentRoots(t *testing.T) {
	accepted, err := ResolveDevServer("http://127.0.0.1:5174", false)
	if err != nil || accepted != "http://127.0.0.1:5174/" {
		t.Fatalf("ResolveDevServer() = %q, %v", accepted, err)
	}
	for _, candidate := range []string{
		"https://127.0.0.1:5174/",
		"http://example.com/",
		"http://localhost:5174/nested",
		"http://user:secret@localhost:5174/",
	} {
		if _, err := ResolveDevServer(candidate, false); err == nil {
			t.Fatalf("ResolveDevServer(%q) accepted an unsafe URL", candidate)
		}
	}
	if _, err := ResolveDevServer("http://localhost:5174/", true); err == nil {
		t.Fatal("production build accepted FRONTEND_DEVSERVER_URL")
	}
}

func TestSecurityMiddlewareSetsDesktopRendererHeaders(t *testing.T) {
	handler := SecurityMiddleware("")(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://wails.localhost/", nil))

	policy := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "default-src 'none'") ||
		!strings.Contains(policy, "connect-src 'self'") ||
		!strings.Contains(policy, "child-src 'none'") ||
		!strings.Contains(policy, "frame-src 'none'") ||
		!strings.Contains(policy, "frame-ancestors 'none'") ||
		!strings.Contains(policy, "script-src 'self';") ||
		strings.Contains(policy, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("Content-Security-Policy = %q", policy)
	}
	if response.Header().Get("Permissions-Policy") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers = %#v", response.Header())
	}
}

func TestDevelopmentSecurityPolicyAllowsOnlyTheLocalViteTransport(t *testing.T) {
	handler := SecurityMiddleware("http://localhost:5174/")(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/", nil))
	policy := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "http://localhost:5174 ws://localhost:5174") || !strings.Contains(policy, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("development Content-Security-Policy = %q", policy)
	}
}

func TestRuntimeRequestsRequireTrustedRendererHeaders(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		origin      string
		contentType string
		clientID    string
		fetchSite   string
		wantStatus  int
	}{
		{name: "same origin", method: http.MethodPost, origin: "http://wails.localhost", contentType: "application/json", clientID: "renderer-client", fetchSite: "same-origin", wantStatus: http.StatusNoContent},
		{name: "missing client id", method: http.MethodPost, origin: "http://wails.localhost", contentType: "application/json", fetchSite: "same-origin", wantStatus: http.StatusForbidden},
		{name: "simple cross origin content type", method: http.MethodPost, origin: "http://evil.example", contentType: "text/plain", clientID: "renderer-client", fetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "mismatched origin", method: http.MethodPost, origin: "http://evil.example", contentType: "application/json", clientID: "renderer-client", fetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "preflight", method: http.MethodOptions, origin: "http://evil.example", contentType: "application/json", clientID: "renderer-client", fetchSite: "cross-site", wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := SecurityMiddleware("")(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(test.method, "http://wails.localhost/wails/runtime", strings.NewReader(`{}`))
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set("X-Wails-Client-Id", test.clientID)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
