package webhook_test

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
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/go-chi/chi/v5"
)

func TestHandlePluginWebhookUsesStaticManifestRegistration(t *testing.T) {
	t.Parallel()
	registry, server, runtime := newStaticWebhookServer(t, 1024)
	body := []byte(`{"action":"opened"}`)
	response := sendSignedWebhook(t, server.URL, body, "gh-evt-test-1")
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
		if raw["body_json"].(map[string]any)["action"] != "opened" {
			t.Fatalf("raw payload = %#v", raw)
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
	response := sendSignedWebhook(t, server.URL, []byte(`{"action":"payload-too-large"}`), "gh-evt-large")
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func newStaticWebhookServer(t *testing.T, maxBodyBytes int) (*pluginwebhook.Registry, *httptest.Server, *testutil.EventRuntime) {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	t.Cleanup(dispatcher.Close)
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID: "repo-watcher", Name: "Repo Watcher", Valid: true,
		RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		Events: []string{"webhook.received"}, Permissions: map[string]plugins.PermissionGrant{"event.raw_payload": {}},
		Webhooks: []plugins.WebhookScope{{
			ID: "github", Route: "github", AuthStrategy: "hmac_sha256", Header: "X-Hub-Signature-256",
			SecretRef: "webhook.github.secret", SignaturePrefix: "sha256=", MaxBodyBytes: maxBodyBytes,
			ReplayProtection: plugins.WebhookReplayProtection{TimestampHeader: "X-Raylea-Timestamp", EventIDHeader: "X-Raylea-Event-Id", ToleranceSeconds: 300, Enforce: true},
		}},
	}})
	registry := pluginwebhook.NewRegistry()

	service, err := pluginwebhook.New(pluginwebhook.Deps{Registry: registry, Plugins: catalog, Secrets: secretStore, Dispatcher: dispatcher, Logger: slog.Default(), Runtime: unexpectedRuntimeStart{t}})
	if err != nil {
		t.Fatal(err)
	}
	service.SyncManifestRegistrations()
	runtime := &testutil.EventRuntime{Events: make(chan chatevent.Event, 1)}
	dispatcher.Register("repo-watcher", runtime, []string{"webhook.received"}, nil, 1)
	if err := secretStore.Set(context.Background(), "webhook.github.secret", []byte("fixture-webhook-secret")); err != nil {
		t.Fatalf("set secret: %v", err)
	}
	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", service.HandleWebhook())
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return registry, server, runtime
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

type unexpectedRuntimeStart struct{ t *testing.T }

func (s unexpectedRuntimeStart) EnsurePluginRunning(context.Context, string) error {
	s.t.Error("webhook attempted to start an already registered runtime")
	return context.Canceled
}
