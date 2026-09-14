package webhook_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/go-chi/chi/v5"
)

func TestHandlePluginWebhookForwardsHeadersAndRawBody(t *testing.T) {
	t.Parallel()
	registry, server, runtime := newStaticWebhookServer(t, 1024)
	body := []byte(`{"action":"opened",  "number": 1}`)
	response := sendWebhook(t, server.URL, body)
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, body = %s", response.StatusCode, payload)
	}
	select {
	case event := <-runtime.Events:
		if event.EventType != "webhook.received" || event.Target == nil || event.Target.ID != "github" {
			t.Fatalf("event = %#v", event)
		}
		raw := event.RawPayload.(map[string]any)
		if raw["body_text"] != string(body) {
			t.Fatalf("raw body = %#v, want the exact request bytes", raw["body_text"])
		}
		headers := raw["headers"].(map[string]any)
		if signature, _ := headers["X-Hub-Signature-256"].([]string); len(signature) != 1 || signature[0] != "sha256=fixture" {
			t.Fatalf("headers = %#v", headers)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook event was not dispatched")
	}
	if _, ok := registry.Get("repo-watcher", "github"); !ok {
		t.Fatal("static webhook registration missing")
	}
}

func TestHandlePluginWebhookRejectsManifestBodyLimit(t *testing.T) {
	t.Parallel()
	_, server, _ := newStaticWebhookServer(t, 16)
	response := sendWebhook(t, server.URL, []byte(`{"action":"payload-too-large"}`))
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func newStaticWebhookServer(t *testing.T, maxBodyBytes int) (*pluginwebhook.Registry, *httptest.Server, *testutil.EventRuntime) {
	t.Helper()
	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	t.Cleanup(dispatcher.Close)
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID: "repo-watcher", Name: "Repo Watcher", Valid: true,
		RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		Events:   []string{"webhook.received"},
		Webhooks: []plugins.WebhookScope{{ID: "github", Route: "github", MaxBodyBytes: maxBodyBytes}},
	}})
	registry := pluginwebhook.NewRegistry()

	service, err := pluginwebhook.New(pluginwebhook.Deps{Registry: registry, Plugins: catalog, Dispatcher: dispatcher, Logger: slog.Default(), Runtime: unexpectedRuntimeStart{t}})
	if err != nil {
		t.Fatal(err)
	}
	service.SyncManifestRegistrations()
	runtime := &testutil.EventRuntime{Events: make(chan chatevent.Event, 1)}
	dispatcher.Register("repo-watcher", runtime, []string{"webhook.received"}, nil, 1)
	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", service.HandleWebhook())
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return registry, server, runtime
}

func sendWebhook(t *testing.T, baseURL string, body []byte) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/webhooks/repo-watcher/github", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Hub-Signature-256", "sha256=fixture")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("send webhook: %v", err)
	}
	return response
}

type unexpectedRuntimeStart struct{ t *testing.T }

func (s unexpectedRuntimeStart) EnsurePluginRunning(context.Context, string) error {
	s.t.Error("webhook attempted to start an already registered runtime")
	return context.Canceled
}
