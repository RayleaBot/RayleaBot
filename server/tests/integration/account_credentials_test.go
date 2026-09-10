package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
)

func TestAccountCredentialChangeExpiresConnectedWebSockets(t *testing.T) {
	application := newTestApp(t)
	token, claims, err := application.AuthManager().Bootstrap("admin", "fixture-old-password")
	if err != nil {
		t.Fatal(err)
	}
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var connections []*websocket.Conn
	for _, path := range []string{"/ws/events", "/ws/logs"} {
		conn, _, err := websocket.Dial(ctx, websocketURL(server.URL)+path, &websocket.DialOptions{
			Host:       testManagementAuthority,
			HTTPHeader: http.Header{"Authorization": {"Bearer " + token}, "Origin": {testManagementOrigin}},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer func(release func() error) { _ = release() }(conn.CloseNow)
		connections = append(connections, conn)
	}
	if err := application.AuthManager().UpdateCredentialsWithContext(ctx, claims, "fixture-old-password", "fixture-new-password", ""); err != nil {
		t.Fatal(err)
	}
	for _, conn := range connections {
		for {
			var frame struct {
				Type string `json:"type"`
			}
			if err := wsjson.Read(ctx, conn, &frame); err != nil {
				t.Fatalf("connection closed without session invalidation: %v", err)
			}
			if frame.Type == "session_expired" {
				break
			}
		}
		for {
			_, _, err := conn.Read(ctx)
			if err == nil {
				continue
			}
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("unexpected connection closure: %v", err)
			}
			break
		}
	}
}

func TestAccountCredentialChangeAdmissionAndReauthentication(t *testing.T) {
	application := newTestApp(t)
	token, claims, err := application.AuthManager().Bootstrap("admin", "fixture-old-password")
	if err != nil {
		t.Fatal(err)
	}
	otherToken, _, err := application.AuthManager().Login("admin", "fixture-old-password")
	if err != nil {
		t.Fatal(err)
	}
	csrf := application.AuthManager().CSRFToken(claims)
	payload := `{"current_secret":"fixture-old-password","new_secret":"fixture-new-password","new_identifier":"new-admin"}`
	request := func(body, token, csrf, origin string, cookie bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPut, "/api/account/credentials", strings.NewReader(body))
		r.Host = "127.0.0.1:8080"
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie {
			r.AddCookie(&http.Cookie{Name: managementapi.SessionCookieName, Value: token})
		} else if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if csrf != "" {
			r.Header.Set(managementapi.CSRFHeader, csrf)
		}
		response := httptest.NewRecorder()
		application.Handler().ServeHTTP(response, r)
		return response
	}
	for _, tc := range []struct {
		name, token, csrf, origin, body string
		cookie                          bool
		want                            int
		code                            string
	}{
		{name: "anonymous", body: payload, want: 401, code: "permission.authentication_required"},
		{name: "cookie without CSRF", token: token, origin: "http://127.0.0.1:8080", body: payload, cookie: true, want: 401, code: "permission.authentication_required"},
		{name: "foreign origin", token: token, csrf: csrf, origin: "https://example.invalid", body: payload, cookie: true, want: 401, code: "permission.authentication_required"},
		{name: "wrong password", token: token, body: `{"current_secret":"wrong","new_secret":"fixture-new-password"}`, want: 403, code: "permission.current_secret_invalid"},
		{name: "short password", token: token, body: `{"current_secret":"fixture-old-password","new_secret":"short"}`, want: 400, code: "platform.invalid_request"},
		{name: "null username", token: token, body: `{"current_secret":"fixture-old-password","new_secret":"fixture-new-password","new_identifier":null}`, want: 400, code: "platform.invalid_request"},
		{name: "unknown field", token: token, body: `{"current_secret":"fixture-old-password","new_secret":"fixture-new-password","admin":true}`, want: 400, code: "platform.invalid_request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := request(tc.body, tc.token, tc.csrf, tc.origin, tc.cookie)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error.Code != tc.code {
				t.Fatalf("code = %s", envelope.Error.Code)
			}
			if bytes.Contains(response.Body.Bytes(), []byte("fixture-")) {
				t.Fatal("response disclosed credentials")
			}
			if _, err := application.AuthManager().Validate(token); err != nil {
				t.Fatalf("failed change revoked session: %v", err)
			}
		})
	}
	response := request(payload, token, csrf, "http://127.0.0.1:8080", true)
	if response.Code != http.StatusNoContent {
		t.Fatalf("change status = %d: %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 || cookies[len(cookies)-1].MaxAge != -1 {
		t.Fatal("browser cookie not cleared")
	}
	for _, old := range []string{token, otherToken} {
		if response := request(payload, old, "", "", false); response.Code != http.StatusUnauthorized {
			t.Fatalf("old session still accepted: %d", response.Code)
		}
	}
	if _, _, err := application.AuthManager().Login("admin", "fixture-old-password"); err == nil {
		t.Fatal("old credentials still work")
	}
	if _, _, err := application.AuthManager().Login("new-admin", "fixture-new-password"); err != nil {
		t.Fatal(err)
	}
}
