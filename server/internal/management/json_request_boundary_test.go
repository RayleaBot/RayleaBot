package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

type requestBoundaryActionInvoker struct {
	payload map[string]any
}

func (invoker *requestBoundaryActionInvoker) InvokeManagementAction(_ context.Context, _, _ string, payload map[string]any) (map[string]any, error) {
	invoker.payload = payload
	return payload, nil
}

func TestManagementJSONRequestBoundaries(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	configRepo, err := pluginstore.NewConfigSQLiteRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatal(err)
	}
	catalog := newTestCatalog([]plugins.Snapshot{{PluginID: "fixture", Valid: true, RegistrationState: "installed"}})
	invoker := &requestBoundaryActionInvoker{}
	ui := NewPluginManagementUIHandlers(PluginManagementUIDeps{
		Plugins: catalog, PluginConfig: configRepo, Secrets: secretStore, ActionInvoker: invoker,
	})
	system := NewSystemHandlers(diagnosticsTestSystem{})
	market := PluginStoreRoutes{Service: emptyPluginStoreService{}}
	installer := &inspectionInstaller{}
	artifactRoot := t.TempDir()
	artifact := filepath.Join(artifactRoot, "artifact")
	if err := os.Mkdir(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	development := DevelopmentRoutes{ArtifactRoot: artifactRoot, Installer: &developmentInstallerStub{}}
	developmentBody, err := json.Marshal(map[string]string{"artifact": artifact, "source": artifactRoot})
	if err != nil {
		t.Fatal(err)
	}
	installBody, err := json.Marshal(trustedInstallRequest())
	if err != nil {
		t.Fatal(err)
	}

	const dynamicValues = `{"custom":{"array":[1,true,null,{"plugin_defined":"value"}]},"flag":false}`
	for _, endpoint := range []struct {
		name       string
		handler    http.HandlerFunc
		body       string
		status     int
		limit      int
		allowEmpty bool
	}{
		{name: "plugin settings", handler: ui.HandlePluginSettingsPut(), body: `{"values":` + dynamicValues + `}`, status: 200},
		{name: "plugin secrets put", handler: ui.HandlePluginSecretsPut(), body: `{"values":{"fixture.key":"fixture-only-secret"}}`, status: 200},
		{name: "plugin secrets delete", handler: ui.HandlePluginSecretsDelete(), body: `{"keys":["fixture.key"]}`, status: 200},
		{name: "plugin action", handler: ui.HandlePluginManagementAction(), body: `{"action":"fixture","payload":` + dynamicValues + `}`, status: 200},
		{name: "plugin inspect", handler: newInstallInspectHandler(catalog, installer), body: `{"source_type":"local_zip","source":"fixture.zip"}`, status: 200},
		{name: "plugin install", handler: newInstallHandler(catalog, installer), body: string(installBody), status: 202},
		{name: "store inspect", handler: market.inspect(), body: `{"source_id":"fixture"}`, status: 200},
		{name: "store install", handler: market.install(), body: string(installBody), status: 202},
		{name: "store create source", handler: market.createSource(), body: `{"name":"fixture","url":"https://example.invalid/catalog.json"}`, status: 201},
		{name: "store update source", handler: market.updateSource(), body: `{"name":"fixture","url":"https://example.invalid/catalog.json"}`, status: 200},
		{name: "recovery confirm", handler: system.HandleSystemRecoveryConfirm(), body: `{"review_ids":["fixture"]}`, status: 202},
		{name: "runtime bootstrap", handler: system.HandleSystemRuntimeBootstrap(), body: `{"resources":["chromium"]}`, status: 202, allowEmpty: true},
		{name: "development sync", handler: development.sync, body: string(developmentBody), status: 200, limit: 16 * 1024},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			limit := endpoint.limit
			if limit == 0 {
				limit = int(httpapi.MaxManagementJSONBodyBytes)
			}
			for _, input := range []struct {
				name   string
				body   string
				status int
			}{
				{name: "oversized object", body: endpoint.body[:len(endpoint.body)-1] + strings.Repeat(" ", limit-len(endpoint.body)+1) + `}`},
				{name: "oversized trailing whitespace", body: endpoint.body + strings.Repeat(" ", limit-len(endpoint.body)+1)},
				{name: "oversized leading whitespace", body: strings.Repeat(" ", limit) + endpoint.body},
				{name: "second object", body: endpoint.body + ` {}`},
				{name: "second null", body: endpoint.body + ` null`},
				{name: "trailing garbage", body: endpoint.body + ` invalid`},
				{name: "unknown field", body: `{"unexpected":true,` + endpoint.body[1:]},
				{name: "null", body: `null`},
				{name: "array", body: `[]`},
				{name: "truncated object", body: endpoint.body[:len(endpoint.body)-1]},
				{name: "empty", status: emptyRequestStatus(endpoint.allowEmpty, endpoint.status)},
				{name: "whitespace", body: " \t\r\n", status: emptyRequestStatus(endpoint.allowEmpty, endpoint.status)},
				{name: "exact limit", body: endpoint.body + strings.Repeat(" ", limit-len(endpoint.body)), status: endpoint.status},
			} {
				t.Run(input.name, func(t *testing.T) {
					beforeValues, err := configRepo.ReadAll(context.Background(), "fixture")
					if err != nil {
						t.Fatal(err)
					}
					beforeSecretKeys, err := secretStore.List(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					beforeActionPayload := invoker.payload
					request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(input.body))
					routeContext := chi.NewRouteContext()
					routeContext.URLParams.Add("plugin_id", "fixture")
					routeContext.URLParams.Add("source_id", "fixture")
					request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
					request = request.WithContext(ContextWithClaims(request.Context(), auth.Claims{Subject: "admin"}))
					response := httptest.NewRecorder()
					endpoint.handler.ServeHTTP(response, request)
					wantStatus := input.status
					if wantStatus == 0 {
						wantStatus = http.StatusBadRequest
					}
					if response.Code != wantStatus {
						t.Fatalf("status = %d, want %d; body = %.500s", response.Code, wantStatus, response.Body.String())
					}
					if wantStatus == http.StatusBadRequest {
						if decodeErrorEnvelope(t, response.Body.Bytes()).Error.Code != "platform.invalid_request" {
							t.Fatalf("unexpected error envelope: %s", response.Body.String())
						}
						afterValues, err := configRepo.ReadAll(context.Background(), "fixture")
						if err != nil {
							t.Fatal(err)
						}
						afterSecretKeys, err := secretStore.List(context.Background())
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(beforeValues, afterValues) || !reflect.DeepEqual(beforeSecretKeys, afterSecretKeys) || !reflect.DeepEqual(beforeActionPayload, invoker.payload) {
							t.Fatal("invalid request changed plugin settings, secrets, or invoked an action")
						}
					}
				})
			}
		})
	}

	var wantValues map[string]any
	if err := json.Unmarshal([]byte(dynamicValues), &wantValues); err != nil {
		t.Fatal(err)
	}
	values, err := configRepo.ReadAll(context.Background(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, wantValues) || !reflect.DeepEqual(invoker.payload, wantValues) {
		t.Fatalf("dynamic JSON was not preserved: settings=%#v action=%#v", values, invoker.payload)
	}
}

func emptyRequestStatus(allowed bool, accepted int) int {
	if allowed {
		return accepted
	}
	return http.StatusBadRequest
}
