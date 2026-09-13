package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestLANBrowserSessionKeepsCookieAndCrossSiteChecks(t *testing.T) {
	t.Parallel()
	application := newTestApp(t, deterministicAuthOptions()...)
	const origin = "http://192.168.1.50:8080"
	setup := httptest.NewRequest(http.MethodPost, origin+"/api/setup/admin", bytes.NewBufferString(`{"identifier":"lan-admin","secret":"fixture-only-password"}`))
	setup.RemoteAddr = "192.168.1.20:32000"
	setup.Header.Set("Content-Type", "application/json")
	setup.Header.Set("Origin", origin)
	setup.Header.Set(managementapi.SetupTokenHeader, testutil.TestSetupToken)
	setup.Header.Set(managementapi.SessionTransportHeader, "cookie")
	response := httptest.NewRecorder()
	application.Handler().ServeHTTP(response, setup)
	if response.Code != http.StatusOK {
		t.Fatalf("LAN initialization failed: status=%d", response.Code)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Secure || cookies[0].Domain != "" {
		t.Fatal("LAN HTTP session must use a host-only HttpOnly cookie without Secure")
	}
	var session struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil || session.CSRF == "" {
		t.Fatalf("session did not provide a CSRF token: %v", err)
	}
	for _, test := range []struct {
		name, path, requestOrigin, csrf string
		want                            int
	}{
		{"missing csrf", "/api/session", origin, "", http.StatusUnauthorized},
		{"cross-site mutation", "/api/session", "http://unrelated.example", session.CSRF, http.StatusUnauthorized},
		{"cross-site websocket", "/ws/events", "http://unrelated.example", "", http.StatusUnauthorized},
		{"same-origin logout", "/api/session", origin, session.CSRF, http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			method := http.MethodDelete
			if test.path == "/ws/events" {
				method = http.MethodGet
			}
			request := httptest.NewRequest(method, origin+test.path, nil)
			request.RemoteAddr = setup.RemoteAddr
			request.Header.Set("Origin", test.requestOrigin)
			request.Header.Set(managementapi.CSRFHeader, test.csrf)
			request.AddCookie(cookies[0])
			recorder := httptest.NewRecorder()
			application.Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status=%d want=%d", recorder.Code, test.want)
			}
		})
	}
}
