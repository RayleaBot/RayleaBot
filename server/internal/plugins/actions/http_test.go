package actions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func TestExecuteHTTPSendsExplicitRequestAndReturnsText(t *testing.T) {
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

	result, err := executeHTTPRequest(context.Background(), "plugin.http", pluginruntime.Action{
		HTTPMethod:  "GET",
		HTTPURL:     server.URL + "/v1/data",
		HTTPHeaders: map[string]string{"X-Request": "fixture"},
	}, config.Config{
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    5,
			MaxRetries:        0,
			AllowPrivateHosts: []string{"127.0.0.1"},
		},
	}, stubHTTPActionCapabilities{
		capabilities: map[string]bool{"http.request": true},
		httpHosts:    []string{"127.0.0.1"},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequest failed: %v", err)
	}
	if result["status_code"] != http.StatusOK || result["body_text"] != "ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteHTTPRejectsUndeclaredHost(t *testing.T) {
	t.Parallel()

	_, err := executeHTTPRequest(context.Background(), "plugin.http", pluginruntime.Action{
		HTTPMethod: "GET",
		HTTPURL:    "https://api.example.test/v1/data",
	}, config.Config{HTTP: config.HTTPConfig{TimeoutSeconds: 5, MaxRetries: 0}}, stubHTTPActionCapabilities{
		capabilities: map[string]bool{"http.request": true},
		httpHosts:    []string{"other.example.test"},
	})

	var runtimeErr *pluginruntime.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.capability_violation" {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

func TestExecuteHTTPMapsOversizedResponseToStableError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("response is too large"))
	}))
	defer server.Close()

	_, err := executeHTTPRequest(context.Background(), "plugin.http", pluginruntime.Action{
		HTTPMethod: "GET",
		HTTPURL:    server.URL,
	}, config.Config{HTTP: config.HTTPConfig{
		TimeoutSeconds:       5,
		MaxResponseBodyBytes: 4,
		AllowPrivateHosts:    []string{"127.0.0.1"},
	}}, stubHTTPActionCapabilities{
		capabilities: map[string]bool{"http.request": true},
		httpHosts:    []string{"127.0.0.1"},
	})

	var runtimeErr *pluginruntime.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "platform.upstream_response_too_large" {
		t.Fatalf("unexpected oversized response error: %#v", err)
	}
}

type stubHTTPActionCapabilities struct {
	capabilities map[string]bool
	httpHosts    []string
}

func (s stubHTTPActionCapabilities) CapabilityDeclared(_ context.Context, _ string, capability string) bool {
	return s.capabilities[capability]
}

func (s stubHTTPActionCapabilities) StorageRootAllowed(context.Context, string, string) bool {
	return false
}

func (s stubHTTPActionCapabilities) HTTPHosts(context.Context, string) []string {
	return append([]string(nil), s.httpHosts...)
}

func (s stubHTTPActionCapabilities) ThirdPartyAccountPlatforms(context.Context, string) []string {
	return nil
}

func (s stubHTTPActionCapabilities) WebhookParameters(context.Context, string, string) (plugins.WebhookScope, bool) {
	return plugins.WebhookScope{}, false
}

func (s stubHTTPActionCapabilities) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}
