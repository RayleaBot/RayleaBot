package pluginapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func TestListHandler_ReturnsPluginMetadata(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Version:           "1.2.3",
		Description:       "Weather query plugin",
		Author:            "raylea",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins", newListHandler(catalog))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp pluginListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(resp.Items))
	}
	if got := resp.Items[0].Version; got != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", got)
	}
	if got := resp.Items[0].Description; got != "Weather query plugin" {
		t.Fatalf("description = %q, want Weather query plugin", got)
	}
	if got := resp.Items[0].Author; got != "raylea" {
		t.Fatalf("author = %q, want raylea", got)
	}
}
