package integration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestSetupAdminReturnsSessionToken(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-admin.yaml"))

	recorder := performJSONRequest(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body)
	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}
	if len(body) != 3 {
		t.Fatalf("unexpected success body shape: %#v", body)
	}

	expected := cloneMap(fixture.Response.Body)
	expected["session_token"] = token
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected success body: got %#v want %#v", body, expected)
	}

	raw := recorder.Body.String()
	if strings.Contains(raw, fixture.Request.Body["identifier"].(string)) || strings.Contains(raw, fixture.Request.Body["secret"].(string)) {
		t.Fatalf("response leaked request credential content: %s", raw)
	}
}

func TestSetupAdminRejectsMalformedRequest(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "invalid.setup-admin-bad-request.yaml"))

	recorder := performJSONRequest(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body)
	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, fixture.Response.Body, "platform.invalid_request")

	raw := recorder.Body.String()
	if strings.Contains(raw, "fixture-only-secret") || strings.Contains(raw, "identifier") && strings.Contains(raw, "admin") {
		t.Fatalf("malformed response leaked request content: %s", raw)
	}
}

func TestSetupAdminRejectsAlreadyInitialized(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	okFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-admin.yaml"))
	edgeFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "edge.setup-admin-already-initialized.yaml"))

	first := performJSONRequest(t, application, okFixture.Request.Method, okFixture.Request.Path, okFixture.Request.Body)
	if first.Code != okFixture.Response.Status {
		t.Fatalf("unexpected first bootstrap status: got %d want %d", first.Code, okFixture.Response.Status)
	}

	second := performJSONRequest(t, application, edgeFixture.Request.Method, edgeFixture.Request.Path, edgeFixture.Request.Body)
	assertCredentialRejection(t, second, edgeFixture, "permission.denied")
}

func TestSetupAdminAllowsLANWithSetupToken(t *testing.T) {
	t.Parallel()
	application := newTestApp(t, deterministicAuthOptions()...)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-admin.yaml"))
	recorder := performJSONRequestWithRemoteAddr(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body, "192.168.1.20:3210")
	if recorder.Code != http.StatusOK {
		t.Fatalf("LAN setup status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSetupAdminUnexpectedAuthFailureReturnsInternalError(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, append(deterministicAuthOptions(), auth.WithRepository(&testutil.StubAuthRepository{
		SaveBootstrapFn: func(context.Context, auth.BootstrapState, auth.Claims) error {
			return errors.New("disk full")
		},
	}))...)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-admin.yaml"))

	recorder := performJSONRequest(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusInternalServerError)
	}

	assertErrorEnvelope(t, decodeBody(t, recorder.Body.Bytes()), "platform.internal_error")
}

// assertErrorEnvelope checks the ErrorEnvelope shape and its stable code; the
// reader-facing message is not part of the contract and is not compared.
func assertErrorEnvelope(t *testing.T, actual map[string]any, wantCode string) {
	t.Helper()
	assertErrorEnvelopeMatchesFixture(t, actual, nil, wantCode)
}

// assertErrorEnvelopeMatchesFixture additionally compares the structured
// details of the envelope against the fixture body when the fixture has any.
func assertErrorEnvelopeMatchesFixture(t *testing.T, actual map[string]any, expected map[string]any, wantCode string) {
	t.Helper()

	errorBody, ok := actual["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %#v", actual)
	}
	if errorBody["code"] != wantCode {
		t.Fatalf("unexpected error code: got %#v want %q", errorBody["code"], wantCode)
	}
	if message, ok := errorBody["message"].(string); !ok || strings.TrimSpace(message) == "" {
		t.Fatalf("unexpected error message: %#v", errorBody["message"])
	}
	requestID, ok := errorBody["request_id"].(string)
	if !ok || !strings.HasPrefix(requestID, "req_") {
		t.Fatalf("unexpected request_id: %#v", errorBody["request_id"])
	}

	expectedError, _ := expected["error"].(map[string]any)
	expectedDetails, hasExpectedDetails := expectedError["details"]
	actualDetails, hasActualDetails := errorBody["details"]
	if hasExpectedDetails != hasActualDetails {
		t.Fatalf("unexpected error details presence: got %#v want %#v", actualDetails, expectedDetails)
	}
	if hasExpectedDetails && !reflect.DeepEqual(actualDetails, expectedDetails) {
		t.Fatalf("unexpected error details: got %#v want %#v", actualDetails, expectedDetails)
	}

	wantLen := 3
	if hasExpectedDetails {
		wantLen = 4
	}
	if len(errorBody) != wantLen {
		t.Fatalf("unexpected error body shape: %#v", errorBody)
	}
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

// assertCredentialRejection checks a rejected credential request against its
// fixture and that the response does not echo the submitted identifier or secret.
func assertCredentialRejection(t *testing.T, recorder *httptest.ResponseRecorder, fixture webAPIFixtureDocument, wantCode string) {
	t.Helper()

	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, fixture.Response.Status)
	}
	assertErrorEnvelopeMatchesFixture(t, decodeBody(t, recorder.Body.Bytes()), fixture.Response.Body, wantCode)
	raw := recorder.Body.String()
	if strings.Contains(raw, fixture.Request.Body["identifier"].(string)) || strings.Contains(raw, fixture.Request.Body["secret"].(string)) {
		t.Fatalf("response leaked request credential content: %s", raw)
	}
}
