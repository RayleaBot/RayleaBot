package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"pgregory.net/rapid"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
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
		auth.WithSigningKey([]byte("property-test-key-0123456789ab")),
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

// issueToken issues a valid token from the given manager for the given subject.
func issueToken(t testingT, manager *auth.Manager, subject string) string {
	token, _, err := manager.Issue(subject)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	return token
}

// dummyHandler is a handler that records that it was called and stores claims from context.
func dummyHandler() (http.Handler, func() bool, func() (auth.Claims, bool)) {
	var called bool
	var gotClaims auth.Claims
	var gotOK bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotClaims, gotOK = app.ClaimsFromContext(r.Context())
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

// Feature: http-auth-middleware, Property 1: Token 提取正确性
// Validates: Requirements 1.1, 1.2
func TestPropertyTokenExtraction(t *testing.T) {
	t.Parallel()

	manager := newPropertyAuthManager(t)
	middleware := app.RequireAuth(manager)

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random token string (non-empty, no leading/trailing whitespace, printable ASCII).
		token := rapid.StringMatching(`[A-Za-z0-9_\-\.]{1,128}`).Draw(t, "token")

		handler, wasCalled, _ := dummyHandler()
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		// The middleware will call Validate on the token. Since these are random strings,
		// Validate will fail. But we can verify the extraction is correct by checking that
		// the middleware attempted validation (returned 401 for invalid token, not for missing token).
		// A more direct test: if we issue a real token and put it in the header, the handler should be called.
		// Let's verify extraction indirectly: the middleware should NOT call the handler for random tokens
		// (they won't validate), confirming it extracted and tried to validate them.
		if wasCalled() {
			// Random tokens should not validate successfully
			t.Fatalf("random token %q should not have validated", token)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for invalid token, got %d", rec.Code)
		}
	})

	// Also verify that valid tokens with Bearer prefix are correctly extracted and validated.
	rapid.Check(t, func(t *rapid.T) {
		manager := newPropertyAuthManagerWithMax(t, 10)
		middleware := app.RequireAuth(manager)

		subject := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, "subject")
		validToken := issueToken(t, manager, subject)

		handler, wasCalled, claimsResult := dummyHandler()
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if !wasCalled() {
			t.Fatalf("handler should have been called for valid token")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for valid token, got %d", rec.Code)
		}

		claims, ok := claimsResult()
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims.Subject != subject {
			t.Fatalf("expected subject %q, got %q", subject, claims.Subject)
		}
	})
}

// Feature: http-auth-middleware, Property 2: 无效鉴权统一拒绝
// Validates: Requirements 1.3, 1.4, 1.5, 2.3, 2.4, 3.4, 4.1, 4.2, 4.3, 4.4
func TestPropertyInvalidAuthUniformRejection(t *testing.T) {
	t.Parallel()

	manager := newPropertyAuthManager(t)
	middleware := app.RequireAuth(manager)

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

		path := rapid.SampledFrom([]string{"/api/config", "/api/logs", "/api/logs/log_test_0001", "/api/plugins", "/api/tasks", "/ws/events", "/ws/tasks", "/ws/logs"}).Draw(t, "path")
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
		if errorObj["code"] != "permission.denied" {
			t.Fatalf("[%s] expected code permission.denied, got %v", sc.name, errorObj["code"])
		}
		if errorObj["message"] != "当前用户无权执行该操作" {
			t.Fatalf("[%s] unexpected message: %v", sc.name, errorObj["message"])
		}
		if errorObj["message_key"] != "errors.permission.denied" {
			t.Fatalf("[%s] unexpected message_key: %v", sc.name, errorObj["message_key"])
		}
		reqID, ok := errorObj["request_id"].(string)
		if !ok || !strings.HasPrefix(reqID, "req_") {
			t.Fatalf("[%s] unexpected request_id: %v", sc.name, errorObj["request_id"])
		}
	})
}

// Feature: http-auth-middleware, Property 3: 请求标识符唯一性
// Validates: Requirements 4.5
func TestPropertyRequestIDUniqueness(t *testing.T) {
	t.Parallel()

	manager := newPropertyAuthManager(t)
	middleware := app.RequireAuth(manager)

	// Collect request_ids from multiple rejection responses and verify uniqueness.
	const batchSize = 50
	rapid.Check(t, func(t *rapid.T) {
		handler, _, _ := dummyHandler()
		wrapped := middleware(handler)

		seen := make(map[string]struct{}, batchSize)
		for i := 0; i < batchSize; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
			// No Authorization header → guaranteed rejection.
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}

			errorObj := parseErrorEnvelope(t, rec.Body.Bytes())
			reqID, ok := errorObj["request_id"].(string)
			if !ok {
				t.Fatalf("request_id not a string: %v", errorObj["request_id"])
			}

			if _, exists := seen[reqID]; exists {
				t.Fatalf("duplicate request_id: %s", reqID)
			}
			seen[reqID] = struct{}{}
		}
	})
}

// Feature: http-auth-middleware, Property 4: 有效 Token 的 Claims 上下文传递
// Validates: Requirements 2.2, 6.1
func TestPropertyValidTokenClaimsContext(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		manager := newPropertyAuthManagerWithMax(t, 10)
		middleware := app.RequireAuth(manager)

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
		if claims.SessionID != expectedClaims.SessionID {
			t.Fatalf("SessionID mismatch: got %q want %q", claims.SessionID, expectedClaims.SessionID)
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

// Feature: http-auth-middleware, Property 5: WebSocket 查询参数备用来源
// Validates: Requirements 5.2
func TestPropertyWebSocketQueryParamFallback(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		manager := newPropertyAuthManagerWithMax(t, 10)
		middleware := app.RequireAuth(manager)

		subject := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, "subject")
		token := issueToken(t, manager, subject)

		handler, wasCalled, claimsResult := dummyHandler()
		wrapped := middleware(handler)

		path := rapid.SampledFrom([]string{"/ws/events", "/ws/tasks", "/ws/logs"}).Draw(t, "path")
		req := httptest.NewRequest(http.MethodGet, path+"?session_token="+token, nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if !wasCalled() {
			t.Fatalf("handler should have been called for valid query param token on %s", path)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		claims, ok := claimsResult()
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims.Subject != subject {
			t.Fatalf("expected subject %q, got %q", subject, claims.Subject)
		}
	})
}

// Feature: http-auth-middleware, Property 6: Authorization 头优先于查询参数
// Validates: Requirements 5.3
func TestPropertyAuthHeaderPrecedence(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		// Create a fresh manager per iteration to avoid hitting max sessions across iterations.
		manager := newPropertyAuthManagerWithMax(t, 10)
		middleware := app.RequireAuth(manager)

		subjectA := rapid.StringMatching(`[a-z]{3,6}`).Draw(t, "subject_a")
		subjectB := rapid.StringMatching(`[a-z]{3,6}`).Draw(t, "subject_b")

		// Ensure subjects are different so we can distinguish which token was used.
		if subjectA == subjectB {
			subjectB = subjectB + "x"
		}

		tokenA := issueToken(t, manager, subjectA)
		tokenB := issueToken(t, manager, subjectB)

		handler, wasCalled, claimsResult := dummyHandler()
		wrapped := middleware(handler)

		path := rapid.SampledFrom([]string{"/ws/events", "/ws/tasks", "/ws/logs"}).Draw(t, "path")
		req := httptest.NewRequest(http.MethodGet, path+"?session_token="+tokenB, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if !wasCalled() {
			t.Fatal("handler should have been called")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		claims, ok := claimsResult()
		if !ok {
			t.Fatal("expected claims in context")
		}
		// Header token (subjectA) should take precedence.
		if claims.Subject != subjectA {
			t.Fatalf("expected header token subject %q, got %q (query param subject was %q)", subjectA, claims.Subject, subjectB)
		}
	})
}

// Feature: http-auth-middleware, Property 7: 未鉴权 Context 返回零值
// Validates: Requirements 6.3
func TestPropertyUnauthenticatedContextZeroValue(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.IntRange(0, 2).Draw(t, "context_kind")

		var ctx context.Context
		switch kind {
		case 0:
			// Empty background context.
			ctx = context.Background()
		case 1:
			// Context with a random unrelated value.
			type randomKey struct{}
			ctx = context.WithValue(context.Background(), randomKey{}, rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "value"))
		case 2:
			// TODO context.
			ctx = context.TODO()
		}

		claims, ok := app.ClaimsFromContext(ctx)
		if ok {
			t.Fatal("expected ok=false for unauthenticated context")
		}

		zeroClaims := auth.Claims{}
		if claims != zeroClaims {
			t.Fatalf("expected zero Claims, got %+v", claims)
		}
	})
}

// ---------------------------------------------------------------------------
// Unit Tests: Route Classification Verification (Task 6.3)
// Validates: Requirements 3.1, 3.2, 3.3, 3.4, 5.1, 5.2
// ---------------------------------------------------------------------------

// TestPublicRoutesAccessibleWithoutToken verifies that all public routes
// return a non-401 status code when no Authorization header is provided.
// Validates: Requirements 3.1, 3.3
func TestPublicRoutesAccessibleWithoutToken(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	publicRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/readyz"},
		{http.MethodGet, "/api/setup/status"},
		{http.MethodPost, "/api/setup/admin"},
		{http.MethodPost, "/api/session/login"},
		{http.MethodPost, "/api/session/launcher-token"},
		{http.MethodPost, "/api/session/launcher-admission"},
	}

	client := server.Client()

	for _, route := range publicRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			url := server.URL + route.path

			var resp *http.Response
			var err error

			switch route.method {
			case http.MethodGet:
				resp, err = client.Get(url)
			case http.MethodPost:
				resp, err = client.Post(url, "application/json", strings.NewReader("{}"))
			}
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusUnauthorized {
				t.Fatalf("public route %s %s returned 401, expected non-401", route.method, route.path)
			}
		})
	}
}

// TestProtectedRoutesReject401WithoutToken verifies that all protected routes
// return 401 with a proper ErrorEnvelope when no Authorization header is provided.
// Validates: Requirements 3.2, 3.4
func TestProtectedRoutesReject401WithoutToken(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodDelete, "/api/session"},
		{http.MethodGet, "/api/config"},
		{http.MethodPut, "/api/config"},
		{http.MethodGet, "/api/system/status"},
		{http.MethodPost, "/api/system/shutdown"},
		{http.MethodGet, "/api/logs"},
		{http.MethodGet, "/api/logs/log_test_0001"},
		{http.MethodGet, "/api/tasks"},
		{http.MethodGet, "/api/plugins"},
		{http.MethodGet, "/api/plugins/fake-plugin-id"},
		{http.MethodGet, "/ws/events"},
		{http.MethodGet, "/ws/tasks"},
		{http.MethodGet, "/ws/logs"},
	}

	client := server.Client()

	for _, route := range protectedRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req, err := http.NewRequest(route.method, server.URL+route.path, nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			// Explicitly no Authorization header.

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("protected route %s %s returned %d, expected 401", route.method, route.path, resp.StatusCode)
			}

			ct := resp.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("expected application/json content-type, got %q", ct)
			}

			var envelope map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
				t.Fatalf("decode response body: %v", err)
			}

			errorObj, ok := envelope["error"].(map[string]any)
			if !ok {
				t.Fatalf("expected error envelope, got %#v", envelope)
			}
			if errorObj["code"] != "permission.denied" {
				t.Fatalf("expected code permission.denied, got %v", errorObj["code"])
			}
			if errorObj["message"] != "当前用户无权执行该操作" {
				t.Fatalf("unexpected message: %v", errorObj["message"])
			}
			if errorObj["message_key"] != "errors.permission.denied" {
				t.Fatalf("unexpected message_key: %v", errorObj["message_key"])
			}
			reqID, ok := errorObj["request_id"].(string)
			if !ok || !strings.HasPrefix(reqID, "req_") {
				t.Fatalf("unexpected request_id: %v", errorObj["request_id"])
			}
		})
	}
}

// TestWebSocketEventsSupportsAuthorizationHeader verifies that /ws/events
// accepts a valid Bearer token via the Authorization header.
// Validates: Requirements 5.1
func TestWebSocketEventsSupportsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wsURL := websocketURL(server.URL) + "/ws/events"
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + token},
		},
	})
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial websocket with Authorization header failed (status %d): %v", status, err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

// TestWebSocketEventsSupportsSessionTokenQueryParam verifies that /ws/events
// accepts a valid token via the session_token query parameter.
// Validates: Requirements 5.2
func TestWebSocketEventsSupportsSessionTokenQueryParam(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wsURL := websocketURL(server.URL) + "/ws/events?session_token=" + token
	conn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial websocket with session_token query param failed (status %d): %v", status, err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func TestAdditionalWebSocketChannelsSupportAuthorizationHeaderAndQueryParam(t *testing.T) {
	t.Parallel()

	paths := []string{"/ws/tasks", "/ws/logs", "/ws/plugins/raylea.help/console"}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			application := newTestApp(t, deterministicAuthOptions()...)
			token := issueLoginToken(t, application)
			server := httptest.NewServer(application.Handler())
			defer server.Close()

			headerCtx, headerCancel := context.WithTimeout(context.Background(), 3*time.Second)
			conn, resp, err := websocket.Dial(headerCtx, websocketURL(server.URL)+path, &websocket.DialOptions{
				HTTPHeader: http.Header{
					"Authorization": []string{"Bearer " + token},
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
			queryConn, queryResp, queryErr := websocket.Dial(queryCtx, websocketURL(server.URL)+path+"?session_token="+token, nil)
			queryCancel()
			if queryErr != nil {
				status := 0
				if queryResp != nil {
					status = queryResp.StatusCode
				}
				t.Fatalf("dial websocket with session_token query param failed (status %d): %v", status, queryErr)
			}
			_ = queryConn.Close(websocket.StatusNormalClosure, "")
		})
	}
}
