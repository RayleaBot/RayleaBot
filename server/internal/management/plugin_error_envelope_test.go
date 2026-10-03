package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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
		}
	}
}
