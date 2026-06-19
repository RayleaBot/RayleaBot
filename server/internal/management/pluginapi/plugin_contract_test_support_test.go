package pluginapi

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func pluginRouter(t *testing.T, catalog *plugincatalog.Catalog) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	RegisterPluginRoutes(router, catalog, nil, nil, nil, nil, nil, nil, nil)
	return router
}

func pluginRouterWithController(t *testing.T, catalog *plugincatalog.Catalog, controller DesiredStateController, uninstaller plugins.UninstallCoordinator) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	RegisterPluginRoutes(router, catalog, nil, nil, nil, controller, uninstaller, nil, nil)
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
	taskID string
	err    error
}

func (s *stubUninstallCoordinator) Accept(_ context.Context, _ string) (string, error) {
	return s.taskID, s.err
}

func assertCommandList(t *testing.T, got any, want []map[string]any) {
	t.Helper()

	items, ok := got.([]any)
	if !ok {
		t.Fatalf("expected commands array, got %#v", got)
	}
	if len(items) != len(want) {
		t.Fatalf("unexpected command count: got %d want %d", len(items), len(want))
	}
	for index, expected := range want {
		command, ok := items[index].(map[string]any)
		if !ok {
			t.Fatalf("expected command object, got %#v", items[index])
		}
		if !reflect.DeepEqual(command, expected) {
			t.Fatalf("unexpected command at index %d: got %#v want %#v", index, command, expected)
		}
	}
}
func assertPluginHelp(t *testing.T, got any, wantTitle string, wantGroup string, wantItemTitle string) {
	t.Helper()

	help, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected help object, got %#v", got)
	}
	if help["title"] != wantTitle {
		t.Fatalf("help.title = %#v, want %q", help["title"], wantTitle)
	}
	groups, ok := help["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("unexpected help groups: %#v", help["groups"])
	}
	group := groups[0].(map[string]any)
	if group["title"] != wantGroup {
		t.Fatalf("help group title = %#v, want %q", group["title"], wantGroup)
	}
	items := group["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected help group items: %#v", group["items"])
	}
	item := items[0].(map[string]any)
	if item["title"] != wantItemTitle {
		t.Fatalf("help item title = %#v, want %q", item["title"], wantItemTitle)
	}
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}

	return body
}
