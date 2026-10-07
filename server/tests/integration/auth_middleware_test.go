package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/coder/websocket"
	"pgregory.net/rapid"
)

// newPropertyAuthManager creates a deterministic auth.Manager for property tests.
func newPropertyAuthManager(t *testing.T) *auth.Manager {
	t.Helper()
	return newPropertyAuthManagerWithMax(t, 100)
}

// newPropertyAuthManagerWithMax creates a deterministic auth.Manager with a custom max sessions limit.
func newPropertyAuthManagerWithMax(t testingT, maxSessions int) *auth.Manager {
	manager, err := auth.NewManager(
		auth.Config{
			SessionTTLDays: 1,
			SlidingRenewal: false,
			MaxSessions:    maxSessions,
		},
		auth.WithClock(func() time.Time {
			return time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
		}),
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager
}

// testingT is a common interface satisfied by both *testing.T and *rapid.T.
type testingT interface {
	Fatalf(format string, args ...any)
}

// dummyHandler is a handler that records that it was called and stores claims from context.
func dummyHandler() (http.Handler, func() bool, func() (auth.Claims, bool)) {
	var called bool
	var gotClaims auth.Claims
	var gotOK bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotClaims, gotOK = managementapi.ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	wasCalled := func() bool { return called }
	claimsResult := func() (auth.Claims, bool) { return gotClaims, gotOK }

	return handler, wasCalled, claimsResult
}

// parseErrorEnvelope parses the response body as an ErrorEnvelope and returns the error object.
func parseErrorEnvelope(t testingT, body []byte) map[string]any {
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("unmarshal error envelope: %v", err)
	}

	errorObj, ok := envelope["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %#v", envelope)
	}

	return errorObj
}

// Every malformed or unknown credential is rejected the same way: a 401 with
// the authentication_required envelope and no call into the handler.
func TestPropertyInvalidAuthUniformRejection(t *testing.T) {
	t.Parallel()

	manager := newPropertyAuthManager(t)
	middleware := managementapi.RequireAuthWithConfig(manager, nil)

	// Scenario generator: one of four invalid auth scenarios.
	type scenario struct {
		name   string
		header string // empty means no Authorization header
	}

	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.IntRange(0, 3).Draw(t, "scenario_kind")

		var sc scenario
		switch kind {
		case 0:
			// No Authorization header
			sc = scenario{name: "no_header", header: ""}
		case 1:
			// Wrong prefix (not "Bearer ")
			prefix := rapid.SampledFrom([]string{"Basic ", "Token ", "bearer ", "BEARER ", "Bear "}).Draw(t, "wrong_prefix")
			token := rapid.StringMatching(`[A-Za-z0-9]{1,64}`).Draw(t, "token")
			sc = scenario{name: "wrong_prefix", header: prefix + token}
		case 2:
			// Empty token after Bearer prefix
			whitespace := rapid.SampledFrom([]string{"", " ", "  ", "\t", " \t "}).Draw(t, "whitespace")
			sc = scenario{name: "empty_token", header: "Bearer " + whitespace}
		case 3:
			// Invalid token (random string that won't validate)
			token := rapid.StringMatching(`[A-Za-z0-9_\-]{1,128}`).Draw(t, "invalid_token")
			sc = scenario{name: "invalid_token", header: "Bearer " + token}
		}

		handler, wasCalled, _ := dummyHandler()
		wrapped := middleware(handler)

		path := rapid.SampledFrom([]string{"/api/config", "/api/logs", "/api/logs/log_test_0001", "/api/plugins", "/ws/events", "/ws/logs"}).Draw(t, "path")
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if sc.header != "" {
			req.Header.Set("Authorization", sc.header)
		}
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		// All invalid scenarios must return 401.
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("[%s] expected 401, got %d", sc.name, rec.Code)
		}

		// Handler must not be called.
		if wasCalled() {
			t.Fatalf("[%s] handler should not have been called", sc.name)
		}

		// Verify Content-Type.
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("[%s] expected application/json content-type, got %q", sc.name, ct)
		}

		// Verify ErrorEnvelope structure.
		errorObj := parseErrorEnvelope(t, rec.Body.Bytes())
		if errorObj["code"] != "permission.authentication_required" {
			t.Fatalf("[%s] expected code permission.authentication_required, got %v", sc.name, errorObj["code"])
		}
		if message, ok := errorObj["message"].(string); !ok || strings.TrimSpace(message) == "" {
			t.Fatalf("[%s] unexpected message: %v", sc.name, errorObj["message"])
		}
		reqID, ok := errorObj["request_id"].(string)
		if !ok || !strings.HasPrefix(reqID, "req_") {
			t.Fatalf("[%s] unexpected request_id: %v", sc.name, errorObj["request_id"])
		}
	})
}

// A valid Bearer token reaches the handler with the issued claims in context.
func TestPropertyValidTokenClaimsContext(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		manager := newPropertyAuthManagerWithMax(t, 10)
		middleware := managementapi.RequireAuthWithConfig(manager, nil)

		subject := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, "subject")
		token, expectedClaims, err := manager.Issue(subject)
		if err != nil {
			t.Fatalf("Issue failed: %v", err)
		}

		handler, wasCalled, claimsResult := dummyHandler()
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if !wasCalled() {
			t.Fatal("handler should have been called for valid token")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		claims, ok := claimsResult()
		if !ok {
			t.Fatal("expected claims in context, got ok=false")
		}
		if claims.TokenHash != expectedClaims.TokenHash {
			t.Fatalf("TokenHash mismatch: got %q want %q", claims.TokenHash, expectedClaims.TokenHash)
		}
		if claims.Subject != expectedClaims.Subject {
			t.Fatalf("Subject mismatch: got %q want %q", claims.Subject, expectedClaims.Subject)
		}
		if !claims.IssuedAt.Equal(expectedClaims.IssuedAt) {
			t.Fatalf("IssuedAt mismatch: got %v want %v", claims.IssuedAt, expectedClaims.IssuedAt)
		}
		if !claims.ExpiresAt.Equal(expectedClaims.ExpiresAt) {
			t.Fatalf("ExpiresAt mismatch: got %v want %v", claims.ExpiresAt, expectedClaims.ExpiresAt)
		}
	})
}

// TestWebSocketEventsRejectsSessionTokenQueryParam verifies that query tokens
// cannot cross the browser trust boundary.
func TestWebSocketEventsRejectsSessionTokenQueryParam(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wsURL := websocketURL(server.URL) + "/ws/events?session_token=" + token
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Host:       testManagementAuthority,
		HTTPHeader: http.Header{"Origin": []string{testManagementOrigin}},
	})
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("query token websocket unexpectedly connected")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("query token websocket status = %#v, want 401", resp)
	}
}

func TestAdditionalWebSocketChannelsSupportAuthorizationHeaderAndRejectQueryParam(t *testing.T) {
	t.Parallel()

	paths := []string{"/ws/logs", "/ws/plugins/raylea.echo/console"}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			application := newTestApp(t, deterministicAuthOptions()...)
			token := issueLoginToken(t, application)
			server := newManagementTestServer(t, application.Handler())
			defer server.Close()

			headerCtx, headerCancel := context.WithTimeout(context.Background(), 3*time.Second)
			conn, resp, err := websocket.Dial(headerCtx, websocketURL(server.URL)+path, &websocket.DialOptions{
				Host: testManagementAuthority,
				HTTPHeader: http.Header{
					"Authorization": []string{"Bearer " + token},
					"Origin":        []string{testManagementOrigin},
				},
			})
			headerCancel()
			if err != nil {
				status := 0
				if resp != nil {
					status = resp.StatusCode
				}
				t.Fatalf("dial websocket with Authorization header failed (status %d): %v", status, err)
			}
			_ = conn.Close(websocket.StatusNormalClosure, "")

			queryCtx, queryCancel := context.WithTimeout(context.Background(), 3*time.Second)
			queryConn, queryResp, queryErr := websocket.Dial(queryCtx, websocketURL(server.URL)+path+"?session_token="+token, &websocket.DialOptions{
				Host:       testManagementAuthority,
				HTTPHeader: http.Header{"Origin": []string{testManagementOrigin}},
			})
			queryCancel()
			if queryErr == nil {
				_ = queryConn.Close(websocket.StatusNormalClosure, "")
				t.Fatal("query token websocket unexpectedly connected")
			}
			if queryResp == nil || queryResp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("query token websocket status = %#v, want 401", queryResp)
			}
		})
	}
}
