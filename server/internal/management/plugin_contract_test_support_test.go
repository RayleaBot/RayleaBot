package management

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/go-chi/chi/v5"
)

func pluginRouter(t *testing.T, catalog *plugincatalog.Catalog) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	registerPluginReadRoutes(router, catalog)
	return router
}

func pluginRouterWithController(t *testing.T, catalog *plugincatalog.Catalog, controller DesiredStateController, uninstaller plugins.UninstallCoordinator) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	registerPluginReadRoutes(router, catalog)
	if controller != nil {
		registerPluginLifecycleRoutes(router, catalog, controller, uninstaller)
	} else {
		router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(catalog, uninstaller))
	}
	return router
}

type stubReloadController struct {
	reloadResult plugins.Snapshot
	reloadErr    error
}

func (s *stubReloadController) Enable(_ context.Context, _ string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}
func (s *stubReloadController) Disable(_ context.Context, _ string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}
func (s *stubReloadController) Reload(_ context.Context, _ string) (plugins.Snapshot, error) {
	return s.reloadResult, s.reloadErr
}
func (s *stubReloadController) RecoverFromDeadLetter(_ context.Context, _ string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}

type stubUninstallCoordinator struct {
	taskID   string
	err      error
	pluginID string
}

func (s *stubUninstallCoordinator) Accept(_ context.Context, pluginID string) (string, error) {
	s.pluginID = pluginID
	return s.taskID, s.err
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}

	return body
}
