package integration

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
)

func TestSessionLoginReturnsSessionToken(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.SetAuthManager(newDeterministicAuthManager(t))

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-login.yaml"))

	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	recorder := performJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if recorder.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, loginFixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}
	if len(body) != 1 {
		t.Fatalf("unexpected success body shape: %#v", body)
	}

	expected := cloneMap(loginFixture.Response.Body)
	expected["session_token"] = token
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected success body: got %#v want %#v", body, expected)
	}

	raw := recorder.Body.String()
	if strings.Contains(raw, loginFixture.Request.Body["identifier"].(string)) || strings.Contains(raw, loginFixture.Request.Body["secret"].(string)) {
		t.Fatalf("response leaked request credential content: %s", raw)
	}
}

func TestSessionLoginRejectsBadCredentials(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.SetAuthManager(newDeterministicAuthManager(t))

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.session-login-bad-credentials.yaml"))

	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	recorder := performJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if recorder.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, loginFixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, loginFixture.Response.Body, "permission.denied")

	raw := recorder.Body.String()
	if strings.Contains(raw, loginFixture.Request.Body["identifier"].(string)) || strings.Contains(raw, loginFixture.Request.Body["secret"].(string)) {
		t.Fatalf("response leaked request credential content: %s", raw)
	}
}

func TestSessionLoginRecyclesOldestSessionWhenMaxSessionsReached(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.SetAuthManager(newLimitedAuthManager(t, 1))

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.session-login-max-sessions.yaml"))

	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}
	bootstrapToken, ok := decodeBody(t, setup.Body.Bytes())["session_token"].(string)
	if !ok || bootstrapToken == "" {
		t.Fatalf("expected bootstrap session token, got %#v", decodeBody(t, setup.Body.Bytes())["session_token"])
	}

	recorder := performJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if recorder.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, loginFixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}
	if len(body) != 1 {
		t.Fatalf("unexpected success body shape: %#v", body)
	}

	expected := cloneMap(loginFixture.Response.Body)
	expected["session_token"] = token
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected success body: got %#v want %#v", body, expected)
	}

	if _, err := application.AuthManager().Validate(bootstrapToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected bootstrap token to be recycled, got %v", err)
	}
	if _, err := application.AuthManager().Validate(token); err != nil {
		t.Fatalf("expected recycled login token to validate, got %v", err)
	}

	raw := recorder.Body.String()
	if strings.Contains(raw, loginFixture.Request.Body["identifier"].(string)) || strings.Contains(raw, loginFixture.Request.Body["secret"].(string)) {
		t.Fatalf("response leaked request credential content: %s", raw)
	}
}

func TestSessionLoginRejectsMalformedRequest(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.SetAuthManager(newDeterministicAuthManager(t))

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	recorder := performJSONRequest(t, application, "POST", "/api/session/login", map[string]any{
		"identifier": "admin",
	})
	if recorder.Code != 400 {
		t.Fatalf("unexpected status: got %d want 400", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, map[string]any{
		"error": map[string]any{
			"code":        "platform.invalid_request",
			"message":     "请求参数不合法",
			"message_key": "errors.platform.invalid_request",
			"request_id":  "fixture_request_id_placeholder",
		},
	}, "platform.invalid_request")
}

func TestSessionLoginRateLimitsAfterRepeatedFailuresFromSameSourceIP(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.SetAuthManager(newDeterministicAuthManager(t))

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.session-login-bad-credentials.yaml"))
	rateLimitedFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.session-login-rate-limited.yaml"))
	okLoginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-login.yaml"))

	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	const blockedRemoteAddr = "203.0.113.10:40123"
	for attempt := 0; attempt < 5; attempt++ {
		recorder := performJSONRequestWithRemoteAddr(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body, blockedRemoteAddr)
		if recorder.Code != loginFixture.Response.Status {
			t.Fatalf("unexpected denied status on attempt %d: got %d want %d", attempt+1, recorder.Code, loginFixture.Response.Status)
		}
	}

	recorder := performJSONRequestWithRemoteAddr(t, application, rateLimitedFixture.Request.Method, rateLimitedFixture.Request.Path, rateLimitedFixture.Request.Body, blockedRemoteAddr)
	if recorder.Code != rateLimitedFixture.Response.Status {
		t.Fatalf("unexpected rate-limited status: got %d want %d", recorder.Code, rateLimitedFixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, rateLimitedFixture.Response.Body, "platform.rate_limited")

	okRecorder := performJSONRequestWithRemoteAddr(t, application, okLoginFixture.Request.Method, okLoginFixture.Request.Path, okLoginFixture.Request.Body, "203.0.113.11:40124")
	if okRecorder.Code != okLoginFixture.Response.Status {
		t.Fatalf("unexpected status for other source IP: got %d want %d", okRecorder.Code, okLoginFixture.Response.Status)
	}
}

func TestSessionLoginRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.SetAuthManager(newDeterministicAuthManager(t))

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	payload := []byte(`{"identifier":"admin","secret":"` + strings.Repeat("a", 2*1024*1024) + `"}`)
	recorder := performJSONBytesRequest(t, application, "POST", "/api/session/login", payload)
	if recorder.Code != 400 {
		t.Fatalf("unexpected status: got %d want 400", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, map[string]any{
		"error": map[string]any{
			"code":        "platform.invalid_request",
			"message":     "请求参数不合法",
			"message_key": "errors.platform.invalid_request",
			"request_id":  "fixture_request_id_placeholder",
		},
	}, "platform.invalid_request")
}

func TestSessionLoginUnexpectedAuthFailureReturnsInternalError(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.SetAuthManager(newDeterministicAuthManagerWithRepository(t, &stubAuthRepository{
		SaveSessionFn: func(context.Context, auth.Claims) error {
			return errors.New("database unavailable")
		},
	}))

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	loginFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-login.yaml"))

	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	recorder := performJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if recorder.Code != 500 {
		t.Fatalf("unexpected status: got %d want 500", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, map[string]any{
		"error": map[string]any{
			"code":        "platform.internal_error",
			"message":     "内部错误",
			"message_key": "errors.platform.internal_error",
			"request_id":  "fixture_request_id_placeholder",
		},
	}, "platform.internal_error")
}

func newLimitedAuthManager(t *testing.T, maxSessions int) *auth.Manager {
	t.Helper()

	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	sessionCounter := 0
	manager, err := auth.NewManager(
		auth.Config{
			SessionTTLDays: 1,
			SlidingRenewal: false,
			MaxSessions:    maxSessions,
		},
		auth.WithClock(func() time.Time {
			return current
		}),
		auth.WithSigningKey([]byte("0123456789abcdef0123456789abcdef")),
		auth.WithSessionIDGenerator(func() (string, error) {
			sessionCounter++
			return "session-test-" + string(rune('0'+sessionCounter)), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager
}
