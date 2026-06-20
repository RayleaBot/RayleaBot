package services

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
)

func TestExecuteHTTPRequestUsesCapabilityedScopeAndReturnsText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/data" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("hello http")); err != nil {
			t.Fatalf("write http response: %v", err)
		}
	}))
	defer server.Close()

	application := newTestAppState(config.Config{
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    5,
			MaxRetries:        0,
			AllowPrivateHosts: []string{"127.0.0.1"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubCapabilityView{
			capabilities: map[string][]stubCapability{
				"scope-cache": {{
					PluginID:   "scope-cache",
					Capability: "http.request",
					ScopeJSON:  `{"http_hosts":["127.0.0.1"]}`,
				}},
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	result, err := application.executeLocalAction(context.Background(), "scope-cache", "req_http_1", runtimeaction.Action{
		Kind:       "http.request",
		HTTPMethod: "GET",
		HTTPURL:    server.URL + "/v1/data",
	})
	if err != nil {
		t.Fatalf("http.request failed: %v", err)
	}
	if got := result["status_code"]; got != http.StatusOK {
		t.Fatalf("unexpected status_code: %#v", got)
	}
	if got := result["body_text"]; got != "hello http" {
		t.Fatalf("unexpected body_text: %#v", got)
	}
}

func TestExecuteHTTPRequestRejectsPrivateHostWithoutAllowlist(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	application := newTestAppState(config.Config{
		HTTP: config.HTTPConfig{
			TimeoutSeconds: 5,
			MaxRetries:     0,
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubCapabilityView{
			capabilities: map[string][]stubCapability{
				"scope-cache": {{
					PluginID:   "scope-cache",
					Capability: "http.request",
					ScopeJSON:  `{"http_hosts":["127.0.0.1"]}`,
				}},
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err := application.executeLocalAction(context.Background(), "scope-cache", "req_http_2", runtimeaction.Action{
		Kind:       "http.request",
		HTTPMethod: "GET",
		HTTPURL:    server.URL + "/v1/data",
	})
	assertRuntimeErrorCode(t, err, "plugin.capability_violation")
}
