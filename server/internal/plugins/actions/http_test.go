package actions_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	defaultactionmodules "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/defaultmodules"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func TestExecuteHTTPRequestReturnsCredentialInjectorError(t *testing.T) {
	t.Parallel()

	service := actions.New(actions.Deps{
		Capabilities: &stubCapabilityView{
			capabilities: map[string]bool{"http.request": true},
			httpHosts:    []string{"api.example.test"},
		},
		HTTPCredentials: stubHTTPCredentials{err: errors.New("sign failed")},
		Registrars:      defaultactionmodules.Registrars(),
	})

	_, err := service.Execute(context.Background(), "raylea.subscription-hub", "req_http_1", runtimeaction.Action{
		Kind:       "http.request",
		HTTPMethod: "GET",
		HTTPURL:    "https://api.example.test/x/polymer/web-dynamic/v1/feed/all",
	}, runtimeprotocol.Event{})

	var runtimeErr *runtimemanager.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.internal_error" || !strings.Contains(runtimeErr.Error(), "sign failed") {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

type stubHTTPCredentials struct {
	err error
}

func (s stubHTTPCredentials) Inject(context.Context, httpaction.CredentialRequest) (httpaction.CredentialResult, error) {
	return httpaction.CredentialResult{}, s.err
}

type stubCapabilityView struct {
	capabilities map[string]bool
	httpHosts    []string
}

func (s *stubCapabilityView) CapabilityDeclared(_ context.Context, _ string, capability string) bool {
	return s.capabilities[capability]
}

func (s *stubCapabilityView) StorageRootAllowed(context.Context, string, string) bool {
	return false
}

func (s *stubCapabilityView) HTTPHosts(context.Context, string) []string {
	return append([]string(nil), s.httpHosts...)
}

func (s *stubCapabilityView) WebhookParameters(context.Context, string, string) (plugins.WebhookScope, bool) {
	return plugins.WebhookScope{}, false
}

func (s *stubCapabilityView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}
