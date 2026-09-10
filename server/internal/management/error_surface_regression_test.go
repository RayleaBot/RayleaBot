package management

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginmarket "github.com/RayleaBot/RayleaBot/server/internal/plugins/market"
)

func TestPluginStoreErrorCausesHaveDistinctHTTPMetadata(t *testing.T) {
	for _, test := range []struct {
		cause  error
		code   string
		status int
	}{
		{pluginmarket.ErrSourceImmutable, errorcodes.PluginStoreSourceImmutable, 409},
		{pluginmarket.ErrSourceConflict, errorcodes.PluginStoreSourceConflict, 409},
		{pluginmarket.ErrSourceInvalid, errorcodes.PlatformInvalidRequest, 400},
		{pluginmarket.ErrSourceNotFound, errorcodes.PlatformResourceNotFound, 404},
	} {
		response := httptest.NewRecorder()
		writePluginStoreError(response, httptest.NewRequest("PUT", "/api/plugin-store/sources/fixture", nil), test.cause)
		body := decodeErrorEnvelope(t, response.Body.Bytes())
		if response.Code != test.status || body.Error.Code != test.code || body.Error.MessageKey != "errors."+test.code {
			t.Fatalf("status=%d body=%s", response.Code, response.Body)
		}
	}
}

type webhookConfigSource struct{ cfg config.Config }

func (s webhookConfigSource) CurrentConfig() config.Config { return s.cfg }

func TestUnavailableWebhookUsesTransportUnavailable(t *testing.T) {
	for _, configured := range []bool{false, true} {
		cfg := config.Config{}
		shells := map[string]*onebot11.Shell{}
		if configured {
			settings := config.OneBotConfig{}
			cfg.Adapters = []config.AdapterInstance{{ID: "onebot11", Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &settings}}
			shells["onebot11"] = onebot11.New("onebot11", settings, config.AdapterConfig{}, nil)
		}
		service := newAdapterTestService(t, webhookConfigSource{cfg: cfg}, adapterservice.Instances{OneBot11: shells})
		handler := NewProtocolHandlers(service)
		router := chi.NewRouter()
		handler.RegisterPublicRoutes(router)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/api/adapters/onebot11/webhook", strings.NewReader("{}")))
		body := decodeErrorEnvelope(t, response.Body.Bytes())
		if response.Code != 503 || body.Error.Code != errorcodes.AdapterTransportUnavailable {
			t.Fatalf("status=%d body=%s", response.Code, response.Body)
		}
	}
}

type failedActionInvoker struct{}

func (failedActionInvoker) InvokeManagementAction(context.Context, string, string, map[string]any) (map[string]any, error) {
	return nil, errors.New("fixture-only-secret")
}

func TestPluginManagementActionFailureIsGatewayFailureWithoutPrivateCause(t *testing.T) {
	handler := NewPluginManagementUIHandlers(PluginManagementUIDeps{Plugins: newTestCatalog([]plugins.Snapshot{{PluginID: "fixture", Valid: true, RegistrationState: "installed"}}), ActionInvoker: failedActionInvoker{}})
	router := chi.NewRouter()
	handler.RegisterProtectedRoutes(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("POST", "/api/plugins/fixture/management/actions", strings.NewReader(`{"action":"fixture"}`)))
	body := decodeErrorEnvelope(t, response.Body.Bytes())
	if response.Code != 502 || body.Error.Code != errorcodes.PluginManagementActionFailed || strings.Contains(response.Body.String(), "fixture-only-secret") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}
