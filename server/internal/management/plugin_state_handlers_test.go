package management

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/go-chi/chi/v5"
)

func TestDesiredStateHandlersDelegateToLifecycle(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"enable", "disable"} {
		for _, tc := range []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{"accepted", nil, 200, ""},
			{"missing", plugins.ErrPluginNotFound, 404, "platform.resource_not_found"},
			{"conflict", plugins.ErrStateConflict, 409, "platform.state_conflict"},
			{"storage failure", errors.New("storage unavailable"), 500, "platform.internal_error"},
		} {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				original := plugins.Snapshot{PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped"}
				catalog := plugincatalog.New([]plugins.Snapshot{original})
				result := original
				result.DesiredState, result.RuntimeState = "enabled", "starting"
				controller := &stubDesiredStateController{enableResult: result, disableResult: result, enableErr: tc.err, disableErr: tc.err}
				router := chi.NewRouter()
				registerPluginLifecycleRoutes(router, catalog, controller, nil)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest("POST", "/api/plugins/fixture/"+action, nil))
				if rec.Code != tc.status {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				if len(controller.calls) != 1 || controller.calls[0] != action+":fixture" {
					t.Fatalf("lifecycle calls=%v", controller.calls)
				}
				if tc.code != "" {
					if got := decodeErrorEnvelope(t, rec.Body.Bytes()).Error.Code; got != tc.code {
						t.Fatalf("code=%s want %s", got, tc.code)
					}
				} else {
					body := decodeBody(t, rec.Body.Bytes())
					if body["plugin"].(map[string]any)["state"] != "starting" {
						t.Fatalf("response did not use lifecycle result: %s", rec.Body.String())
					}
				}
				current, _ := catalog.Get("fixture")
				if current.DesiredState != original.DesiredState {
					t.Fatal("handler mutated catalog")
				}
			})
		}
	}
}

func TestPluginRoutesRejectMissingLifecycleDuringAssembly(t *testing.T) {
	t.Parallel()
	routes, err := NewPluginRoutes(PluginRouteDeps{
		Catalog: plugincatalog.New(nil), Installer: testInstallCoordinator{}, Uninstaller: &stubUninstallCoordinator{},
	})
	if err == nil || routes != nil {
		t.Fatal("incomplete plugin routes must fail assembly")
	}
}
