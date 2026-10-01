package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func TestErrorResponseUsesRegisteredHTTPMetadata(t *testing.T) {
	for _, code := range []string{
		errorcodes.PlatformResourceNotFound, errorcodes.PlatformResourceMissing,
		errorcodes.PlatformStateConflict, errorcodes.PermissionAuthenticationRequired,
		errorcodes.PermissionDenied, errorcodes.PluginManagementActionFailed,
	} {
		t.Run(code, func(t *testing.T) {
			response := httptest.NewRecorder()
			WriteError(response, httptest.NewRequest("GET", "/", nil), code, nil)
			definition, ok := errorcodes.HTTP(code)
			if !ok {
				t.Fatal("test code is not an HTTP error")
			}
			var body ErrorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != definition.HTTPStatus || body.Error.Code != code || body.Error.Message != definition.Message || body.Error.RequestID == "" {
				t.Fatalf("response status=%d, body=%+v", response.Code, body)
			}
		})
	}
}

func TestUndeclaredOrWrongSurfaceErrorDoesNotLeakDetails(t *testing.T) {
	for _, code := range []string{"unknown.code", errorcodes.PluginNotHandled, ""} {
		response := httptest.NewRecorder()
		WriteErrorWithMessage(response, httptest.NewRequest("GET", "/", nil), code, "fixture-only-secret", map[string]any{"private": "fixture-only-secret"})
		var body ErrorEnvelope
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 500 || body.Error.Code != errorcodes.PlatformInternalError || len(body.Error.Details) != 0 || body.Error.Message == "fixture-only-secret" {
			t.Fatalf("invalid error escaped HTTP boundary: %d %+v", response.Code, body)
		}
	}
}
