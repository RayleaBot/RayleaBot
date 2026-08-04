package management_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginmarket"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
)

func TestRegisterManagementRoutes(t *testing.T) {
	router := chi.NewRouter()
	pluginUI := managementapi.NewPluginManagementUIHandlers(managementapi.PluginManagementUIDeps{})
	noopHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	managementapi.RegisterRoutes(router, managementapi.RouteDeps{
		PublicRoutes: []managementapi.PublicRouteModule{
			managementapi.NewAuthHandlers(managementapi.AuthDeps{}),
			managementapi.NewCoreHandlers(managementapi.CoreDeps{}),
			managementapi.NewProtocolHandlers(nil),
			pluginwebhook.New(pluginwebhook.Deps{}),
			pluginUI,
		},
		ProtectedRoutes: []managementapi.ProtectedRouteModule{
			managementapi.NewCoreHandlers(managementapi.CoreDeps{}),
			managementapi.NewConfigHandlers(nil),
			managementapi.NewProtocolHandlers(nil),
			managementapi.NewGovernanceHandlersWithService(nil),
			managementapi.NewLogHandlers(nil),
			managementapi.NewSystemRoutes(managementapi.NewSystemHandlers(nil), noopHandler),
			managementapi.NewRenderHandlers(nil),
			managementapi.NewThirdPartyHandlers(nil, nil, nil),
			managementapi.NewUpdateHandlers(nil),
			pluginUI,
			managementapi.ProtectedRouteFunc(func(r chi.Router) {
				r.Get("/ws/events", noopHandler)
				r.Get("/ws/logs", noopHandler)
				r.Get("/ws/plugins/{id}/console", noopHandler)
			}),
			managementapi.PluginRouteDeps{},
			managementapi.PluginStoreRoutes{Service: emptyPluginStoreService{}},
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

type emptyPluginStoreService struct{}

func (emptyPluginStoreService) List(pluginmarket.Query) pluginmarket.ListResult {
	return pluginmarket.ListResult{}
}
func (emptyPluginStoreService) Get(string) (pluginmarket.DetailResult, bool) {
	return pluginmarket.DetailResult{}, false
}
func (emptyPluginStoreService) Refresh(context.Context) (pluginmarket.CatalogStatus, error) {
	return pluginmarket.CatalogStatus{}, nil
}
func (emptyPluginStoreService) Install(context.Context, pluginmarket.InstallRequest) (string, error) {
	return "", nil
}

func expectedRoutesFromContracts(t *testing.T) []string {
	t.Helper()

	routes := map[string]struct{}{}
	addOpenAPIRoutes(t, routes)
	addWebSocketRoutes(t, routes)

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

func readContractYAML(t *testing.T, name string, out any) {
	t.Helper()

	path := filepath.Join("..", "..", "..", "contracts", name)
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
