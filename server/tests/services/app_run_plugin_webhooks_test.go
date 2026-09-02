package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

type capturingRuntime struct{ events chan pluginruntime.Event }

func (r *capturingRuntime) DeliverEvent(_ context.Context, event pluginruntime.Event) (pluginruntime.Delivery, error) {
	r.events <- event
	return pluginruntime.Delivery{RequestID: "event_webhook_1", Result: map[string]any{}}, nil
}

func (r *capturingRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

func TestHandlePluginWebhookUsesStaticManifestRegistration(t *testing.T) {
	t.Parallel()
	application, server, runtime := newStaticWebhookHarness(t, 1024)
	body := []byte(`{"action":"opened"}`)
	response := sendSignedWebhook(t, server.URL, body, "gh-evt-test-1")
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, body = %s", response.StatusCode, payload)
	}
	select {
	case event := <-runtime.events:
		if event.EventType != "webhook.received" || event.Target == nil || event.Target.ID != "github" {
			t.Fatalf("event = %#v", event)
		}
		raw := event.RawPayload.(map[string]any)
		if raw["body_json"].(map[string]any)["action"] != "opened" {
			t.Fatalf("raw payload = %#v", raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook event was not dispatched")
	}
	if _, ok := application.pluginStack.Webhooks.Get("repo-watcher", "github"); !ok {
		t.Fatal("static webhook registration missing")
	}
}

func TestHandlePluginWebhookRejectsManifestBodyLimit(t *testing.T) {
	t.Parallel()
	_, server, _ := newStaticWebhookHarness(t, 16)
	response := sendSignedWebhook(t, server.URL, []byte(`{"action":"payload-too-large"}`), "gh-evt-large")
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func newStaticWebhookHarness(t *testing.T, maxBodyBytes int) (*serviceHarness, *httptest.Server, *capturingRuntime) {
	t.Helper()
	store, err := storage.Open(t.TempDir() + "\\state.db")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	application := newTestAppState(config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID: "repo-watcher", Name: "Repo Watcher", Valid: true,
		RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		Events: []string{"webhook.received"}, Permissions: map[string]plugins.PermissionGrant{"event.raw_payload": {}},
		Webhooks: []plugins.WebhookScope{{
			ID: "github", Route: "github", AuthStrategy: "hmac_sha256", Header: "X-Hub-Signature-256",
			SecretRef: "webhook.github.secret", SignaturePrefix: "sha256=", MaxBodyBytes: maxBodyBytes,
			ReplayProtection: plugins.WebhookReplayProtection{TimestampHeader: "X-Raylea-Timestamp", EventIDHeader: "X-Raylea-Event-Id", ToleranceSeconds: 300, Enforce: true},
		}},
	}})
	registry := newPluginWebhookRegistry()
	application.setTestLocalActions(&stubPermissionView{permissions: map[string][]stubPermission{}}, nil, nil, nil, nil, dispatcher, nil, nil, nil, nil)
	application.setTestWebhookService(secretStore, dispatcher, nil, registry)
	application.services.PluginWebhooks.SyncManifestRegistrations()
	runtime := &capturingRuntime{events: make(chan pluginruntime.Event, 1)}
	dispatcher.Register("repo-watcher", runtime, []string{"webhook.received"}, nil, 1)
	if err := application.platform.Secrets.Set(context.Background(), "webhook.github.secret", []byte("fixture-webhook-secret")); err != nil {
		t.Fatalf("set secret: %v", err)
	}
	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", application.handlePluginWebhook())
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return application, server, runtime
}

func sendSignedWebhook(t *testing.T, baseURL string, body []byte, eventID string) *http.Response {
	t.Helper()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte("fixture-webhook-secret"))
	_, _ = mac.Write([]byte(timestamp + "\n" + eventID + "\n"))
	_, _ = mac.Write(body)
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/webhooks/repo-watcher/github", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Raylea-Timestamp", timestamp)
	request.Header.Set("X-Raylea-Event-Id", eventID)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("send webhook: %v", err)
	}
	return response
}
