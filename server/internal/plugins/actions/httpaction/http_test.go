package httpaction

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func TestExecuteSendsExplicitRequestAndReturnsText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request"); got != "fixture" {
			t.Fatalf("unexpected explicit header: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	result, err := Execute(context.Background(), Request{
		PluginID: "plugin.http",
		Action: runtimeaction.Action{
			HTTPMethod:  "GET",
			HTTPURL:     server.URL + "/v1/data",
			HTTPHeaders: map[string]string{"X-Request": "fixture"},
		},
		Config: config.Config{
			HTTP: config.HTTPConfig{
				TimeoutSeconds:    5,
				MaxRetries:        0,
				AllowPrivateHosts: []string{"127.0.0.1"},
			},
		},
		Capabilities: stubHTTPCapabilities{
			capabilities: map[string]bool{"http.request": true},
			httpHosts:    []string{"127.0.0.1"},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result["status_code"] != http.StatusOK || result["body_text"] != "ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteRejectsUndeclaredHost(t *testing.T) {
	t.Parallel()

	_, err := Execute(context.Background(), Request{
		PluginID: "plugin.http",
		Action: runtimeaction.Action{
			HTTPMethod: "GET",
			HTTPURL:    "https://api.example.test/v1/data",
		},
		Config: config.Config{HTTP: config.HTTPConfig{TimeoutSeconds: 5, MaxRetries: 0}},
		Capabilities: stubHTTPCapabilities{
			capabilities: map[string]bool{"http.request": true},
			httpHosts:    []string{"other.example.test"},
		},
	})

	var runtimeErr *runtimemanager.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.capability_violation" {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

type stubHTTPCapabilities struct {
	capabilities map[string]bool
	httpHosts    []string
}

func (s stubHTTPCapabilities) CapabilityDeclared(_ context.Context, _ string, capability string) bool {
	return s.capabilities[capability]
}

func (s stubHTTPCapabilities) HTTPHosts(context.Context, string) []string {
	return append([]string(nil), s.httpHosts...)
}
