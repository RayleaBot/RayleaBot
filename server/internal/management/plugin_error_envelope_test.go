package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func TestInvalidInstallRequestsReturnExpectedErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"malformed JSON", `{invalid`, "platform.invalid_request", 400},
		{"untrusted source", `{"source_type":"local_zip","source":"C:/plugins/weather.zip","trusted_code_confirmed":false}`, "plugin.trusted_code_confirmation_required", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Post("/api/plugins/install", newInstallHandler(nil))
			req := httptest.NewRequest("POST", "/api/plugins/install", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			env := decodeErrorEnvelope(t, rec.Body.Bytes())
			if rec.Code != tc.status || env.Error.Code != tc.code || env.Error.RequestID == "" {
				t.Fatalf("invalid error response: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestInstallCoreVersionErrorsPreserveReason(t *testing.T) {
	for _, tc := range []struct {
		coreVersion string
		reason      string
	}{
		{coreVersion: "unknown", reason: "core_version_unknown"},
		{coreVersion: "0.3.0", reason: "core_version_too_old"},
	} {
		for _, writer := range []func(http.ResponseWriter, *http.Request, error){writePluginInstallError, writePluginStoreError} {
			cause := plugins.CheckCoreVersion(tc.coreVersion, "0.4.0")
			response := httptest.NewRecorder()
			writer(response, httptest.NewRequest("POST", "/", nil), fmt.Errorf("admission: %w", cause))
			var body httpapi.ErrorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusConflict || body.Error.Code != errorcodes.PluginCoreVersionIncompatible || body.Error.RequestID == "" {
				t.Fatalf("unexpected error identity: %d %s", response.Code, response.Body)
			}
			if body.Error.Details["incompatible_reason"] != tc.reason || body.Error.Details["min_core_version"] != "0.4.0" || len(body.Error.Details) != 2 {
				t.Fatalf("unexpected error details: %#v", body.Error.Details)
			}
			if tc.reason == "core_version_unknown" && (!strings.Contains(body.Error.Message, "无法确认当前 RayleaBot 版本") || strings.Contains(body.Error.Message, "unknown")) {
				t.Fatalf("unknown version message = %q", body.Error.Message)
			}
		}
	}
}
