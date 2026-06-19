package pluginapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
	"pgregory.net/rapid"
)

func TestProperty_ErrorResponseSchemaConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Build a catalog with one installed+enabled plugin for 409 scenarios.
		catalog := newTestCatalog([]plugins.Snapshot{{
			PluginID:          "existing",
			Name:              "Existing Plugin",
			Version:           "1.0.0",
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Valid:             true,
		}})
		taskRegistry := tasks.NewRegistry()
		repo := &stubDesiredStateRepository{}
		router := chi.NewRouter()
		router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, nil))
		router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, nil, nil, nil))
		router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, nil, nil, nil))

		// Pick one of several error-triggering scenarios.
		scenario := rapid.IntRange(0, 3).Draw(t, "scenario")
		var req *http.Request
		switch scenario {
		case 0: // 400 — malformed install body
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(`{invalid`))
			req.Header.Set("Content-Type", "application/json")
		case 1: // 400 — empty source
			b, _ := json.Marshal(pluginInstallRequest{SourceType: "local_zip", Source: ""})
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		case 2: // 404 — non-existent plugin enable
			fakeID := rapid.StringMatching("[a-z]{5,15}").
				Filter(func(s string) bool { return s != "existing" }).
				Draw(t, "fakeID")
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/"+fakeID+"/enable", nil)
		case 3: // 409 — already enabled
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/existing/enable", nil)
		}

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Verify Content-Type.
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("scenario=%d Content-Type = %q, want application/json", scenario, ct)
		}

		// Verify ErrorEnvelope schema.
		env := decodeErrorEnvelope(t, rec.Body.Bytes())
		if env.Error.Code == "" {
			t.Fatalf("scenario=%d error.code is empty", scenario)
		}
		if env.Error.Message == "" {
			t.Fatalf("scenario=%d error.message is empty", scenario)
		}
		if env.Error.MessageKey == "" {
			t.Fatalf("scenario=%d error.message_key is empty", scenario)
		}
		if env.Error.RequestID == "" {
			t.Fatalf("scenario=%d error.request_id is empty", scenario)
		}
	})
}
