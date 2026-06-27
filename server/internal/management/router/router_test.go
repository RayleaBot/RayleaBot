package router

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
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
			thirdpartyapi.NewThirdPartyHandlers(nil, nil, nil),
			pluginUI,
			ProtectedRouteFunc(func(r chi.Router) {
				r.Get("/ws/events", noopHandler)
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

	want := expectedRoutesFromContracts(t)
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

func expectedRoutesFromContracts(t *testing.T) []string {
	t.Helper()

	routes := map[string]struct{}{}
	addOpenAPIRoutes(t, routes)
	addWebSocketRoutes(t, routes)
	addPluginUIRoutes(t, routes)

	result := make([]string, 0, len(routes))
	for route := range routes {
		result = append(result, route)
	}
	return result
}

func addOpenAPIRoutes(t *testing.T, routes map[string]struct{}) {
	t.Helper()

	var document struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	readContractYAML(t, "web-api.openapi.yaml", &document)

	for path, operations := range document.Paths {
		for method := range operations {
			if !isHTTPMethod(method) {
				continue
			}
			routes[strings.ToUpper(method)+" "+path] = struct{}{}
		}
	}
}

func addWebSocketRoutes(t *testing.T, routes map[string]struct{}) {
	t.Helper()

	var document struct {
		Channels []struct {
			Path string `yaml:"path"`
		} `yaml:"channels"`
	}
	readContractYAML(t, "websocket-events.yaml", &document)

	for _, channel := range document.Channels {
		if channel.Path == "" {
			continue
		}
		routes["GET "+channel.Path] = struct{}{}
	}
}

func addPluginUIRoutes(t *testing.T, routes map[string]struct{}) {
	t.Helper()

	var document struct {
		StaticRoute struct {
			PathTemplate string   `yaml:"path_template"`
			Methods      []string `yaml:"methods"`
		} `yaml:"static_route"`
	}
	readContractYAML(t, "plugin-management-ui.yaml", &document)

	path := strings.TrimSpace(document.StaticRoute.PathTemplate)
	path = strings.Replace(path, "{asset_path}", "*", 1)
	for _, method := range document.StaticRoute.Methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method == "" {
			continue
		}
		routes[method+" "+path] = struct{}{}
	}
}

func readContractYAML(t *testing.T, name string, out any) {
	t.Helper()

	path := filepath.Join("..", "..", "..", "..", "contracts", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := yaml.Unmarshal(raw, out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

func isHTTPMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "delete", "get", "head", "patch", "post", "put":
		return true
	default:
		return false
	}
}
