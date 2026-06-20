package router

import (
	"net/http"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/bilibiliapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/taskapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/thirdpartyapi"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
)

func TestRegisterManagementRoutes(t *testing.T) {
	router := chi.NewRouter()
	pluginUI := pluginui.NewHandlers(pluginui.Deps{})
	noopHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	Register(router, Deps{
		PublicRoutes: []PublicRouteModule{
			authapi.NewHandlers(authapi.Deps{}),
			coreapi.NewHandlers(coreapi.Deps{}),
			protocolapi.NewHandlers(nil),
			pluginwebhook.New(pluginwebhook.Deps{}),
			pluginUI,
		},
		ProtectedRoutes: []ProtectedRouteModule{
			coreapi.NewHandlers(coreapi.Deps{}),
			configapi.NewHandlers(nil),
			protocolapi.NewHandlers(nil),
			governanceapi.NewHandlersWithService(nil),
			logapi.NewHandlers(nil),
			systemapi.NewRoutes(systemapi.NewSystemHandlers(nil), noopHandler),
			renderapi.NewHandlers(nil),
			thirdpartyapi.NewThirdPartyHandlers(nil, nil, nil, nil),
			bilibiliapi.NewBilibiliHandlers(nil, nil, nil),
			taskapi.NewHandlers(nil, nil, nil),
			pluginUI,
			ProtectedRouteFunc(func(r chi.Router) {
				r.Get("/ws/events", noopHandler)
				r.Get("/ws/tasks", noopHandler)
				r.Get("/ws/logs", noopHandler)
				r.Get("/ws/plugins/{id}/console", noopHandler)
			}),
			pluginapi.RouteDeps{},
		},
	}, func(next http.Handler) http.Handler {
		return next
	})

	var got []string
	if err := chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got = append(got, method+" "+route)
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	sort.Strings(got)

	want := []string{
		"DELETE /api/governance/blacklist/entries/{entry_type}/{target_id}",
		"DELETE /api/governance/whitelist/entries/{entry_type}/{target_id}",
		"DELETE /api/plugins/{plugin_id}",
		"DELETE /api/session",
		"DELETE /api/third-party/accounts/{platform}/{account_id}",
		"GET /api/bilibili/login/qrcode/{login_id}",
		"GET /api/bilibili/source/status",
		"GET /api/bilibili/users/resolve",
		"GET /api/config",
		"GET /api/governance/blacklist",
		"GET /api/governance/command-policy",
		"GET /api/governance/whitelist",
		"GET /api/launcher/status",
		"GET /api/logs",
		"GET /api/logs/{log_id}",
		"GET /api/plugins",
		"GET /api/plugins/{plugin_id}",
		"GET /api/plugins/{plugin_id}/secrets",
		"GET /api/plugins/{plugin_id}/settings",
		"GET /api/protocols/onebot11",
		"GET /api/protocols/onebot11/compatibility",
		"GET /api/protocols/onebot11/reverse-ws",
		"GET /api/protocols/onebot11/targets",
		"GET /api/setup/status",
		"GET /api/system/diagnostics/export",
		"GET /api/system/metrics",
		"GET /api/system/render/templates",
		"GET /api/system/render/templates/{template_id}",
		"GET /api/system/render/templates/{template_id}/asset",
		"GET /api/system/scheduler/jobs",
		"GET /api/system/status",
		"GET /api/tasks",
		"GET /api/tasks/{task_id}",
		"GET /api/third-party/accounts",
		"GET /api/third-party/media",
		"GET /api/third-party/monitors",
		"GET /healthz",
		"GET /plugin-ui/{plugin_id}/*",
		"GET /readyz",
		"GET /ws/events",
		"GET /ws/logs",
		"GET /ws/plugins/{id}/console",
		"GET /ws/tasks",
		"HEAD /plugin-ui/{plugin_id}/*",
		"POST /api/bilibili/login/qrcode",
		"POST /api/bilibili/source/restart",
		"POST /api/governance/blacklist/entries",
		"POST /api/governance/whitelist/entries",
		"POST /api/launcher/shutdown",
		"POST /api/plugins/{plugin_id}/disable",
		"POST /api/plugins/{plugin_id}/enable",
		"POST /api/plugins/{plugin_id}/reload",
		"POST /api/plugins/{plugin_id}/dead_letter/recover",
		"POST /api/plugins/install",
		"POST /api/protocols/onebot11/identities/resolve",
		"POST /api/protocols/onebot11/webhook",
		"POST /api/session/login",
		"POST /api/setup/admin",
		"POST /api/system/backup",
		"POST /api/system/recovery/confirm",
		"POST /api/system/recovery/recheck",
		"POST /api/system/render/templates/{template_id}/preview-html",
		"POST /api/system/runtime/bootstrap",
		"POST /api/system/scheduler/jobs/{job_id}/trigger",
		"POST /api/system/shutdown",
		"POST /api/tasks/{task_id}/cancel",
		"POST /api/webhooks/{plugin_id}/{route}",
		"PUT /api/config",
		"PUT /api/governance/whitelist/state",
		"PUT /api/plugins/{plugin_id}/secrets",
		"PUT /api/plugins/{plugin_id}/settings",
		"PUT /api/third-party/accounts/{platform}/{account_id}",
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("route count mismatch: got %d want %d\nroutes: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("route %d mismatch: got %q want %q\nroutes: %#v", i, got[i], want[i], got)
		}
	}
}
