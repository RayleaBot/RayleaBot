package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

type capturingRuntime struct {
	events chan runtimeprotocol.Event
}

func (r *capturingRuntime) DeliverEvent(_ context.Context, event runtimeprotocol.Event) (runtimemanager.Delivery, error) {
	select {
	case r.events <- event:
	default:
	}
	return runtimemanager.Delivery{
		RequestID: "event_webhook_1",
		Result:    map[string]any{},
	}, nil
}

func (r *capturingRuntime) Snapshot() runtimemanager.Snapshot {
	return runtimemanager.Snapshot{State: runtimemanager.StateRunning}
}

func TestHandlePluginWebhookAcceptsSignedRequestAndDispatchesEvent(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(t.TempDir() + "\\state.db")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	registry := newPluginWebhookRegistry()
	grantRepo := &stubLifecycleGrantRepository{
		grants: map[string][]plugins.PluginGrant{
			"repo-watcher": {{
				PluginID:   "repo-watcher",
				Capability: "event.expose_webhook",
				ScopeJSON:  `{"webhooks":[{"route":"github","auth_strategy":"hmac_sha256","header":"X-Hub-Signature-256","secret_ref":"webhook.github.secret"}]}`,
			}, {
				PluginID:   "repo-watcher",
				Capability: "event.raw_payload",
			}},
		},
	}
	application := newTestAppState(config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"event.expose_webhook", "event.raw_payload"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{PluginID: "repo-watcher", Name: "Repo Watcher", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running"}})
	application.setTestLocalActions(grantRepo, nil, nil, nil, nil, dispatcher, nil, nil, nil, nil)
	application.setTestWebhookService(secretStore, dispatcher, nil, registry)

	fakeRuntime := &capturingRuntime{events: make(chan runtimeprotocol.Event, 1)}
	application.pluginStack.Dispatcher.Register("repo-watcher", fakeRuntime, []string{"webhook.received"}, nil, 1)

	if err := application.platform.Secrets.Set(context.Background(), "webhook.github.secret", []byte("fixture-webhook-secret")); err != nil {
		t.Fatalf("set webhook secret: %v", err)
	}

	if _, err := application.executeLocalAction(context.Background(), "repo-watcher", "req_webhook_register", runtimeaction.Action{
		Kind:                   "event.expose_webhook",
		WebhookRoute:           "github",
		WebhookMethods:         []string{"POST"},
		WebhookAuthStrategy:    "hmac_sha256",
		WebhookHeader:          "X-Hub-Signature-256",
		WebhookSecretRef:       "webhook.github.secret",
		WebhookSignaturePrefix: "sha256=",
		WebhookReplayProtection: &runtimeaction.WebhookReplayProtection{
			TimestampHeader:  "X-Raylea-Timestamp",
			EventIDHeader:    "X-Raylea-Event-Id",
			ToleranceSeconds: 300,
			Enforce:          true,
		},
	}); err != nil {
		t.Fatalf("register webhook action: %v", err)
	}

	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", application.handlePluginWebhook())
	server := httptest.NewServer(router)
	defer server.Close()

	body := []byte(`{"action":"opened"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	eventID := "gh-evt-test-1"
	mac := hmac.New(sha256.New, []byte("fixture-webhook-secret"))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(eventID))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/webhooks/repo-watcher/github", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Raylea-Timestamp", timestamp)
	request.Header.Set("X-Raylea-Event-Id", eventID)
	request.Header.Set("X-Hub-Signature-256", signature)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform webhook request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected webhook status: got %d want %d", response.StatusCode, http.StatusAccepted)
	}

	select {
	case event := <-fakeRuntime.events:
		if event.EventType != "webhook.received" || event.Target == nil || event.Target.ID != "github" {
			t.Fatalf("unexpected webhook event: %#v", event)
		}
		rawPayload, ok := event.RawPayload.(map[string]any)
		if !ok {
			t.Fatalf("expected raw payload map, got %#v", event.RawPayload)
		}
		bodyJSON, ok := rawPayload["body_json"].(map[string]any)
		if !ok || bodyJSON["action"] != "opened" {
			t.Fatalf("unexpected webhook raw payload: %#v", rawPayload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected webhook event to be dispatched")
	}
}

func TestHandlePluginWebhookRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(t.TempDir() + "\\state.db")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	registry := newPluginWebhookRegistry()
	grantRepo := &stubLifecycleGrantRepository{
		grants: map[string][]plugins.PluginGrant{
			"repo-watcher": {{
				PluginID:   "repo-watcher",
				Capability: "event.expose_webhook",
				ScopeJSON:  `{"webhooks":[{"route":"github","auth_strategy":"hmac_sha256","header":"X-Hub-Signature-256","secret_ref":"webhook.github.secret"}]}`,
			}, {
				PluginID:   "repo-watcher",
				Capability: "event.raw_payload",
			}},
		},
	}
	application := newTestAppState(config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"event.expose_webhook", "event.raw_payload"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{PluginID: "repo-watcher", Name: "Repo Watcher", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running"}})
	application.setTestLocalActions(grantRepo, nil, nil, nil, nil, dispatcher, nil, nil, nil, nil)
	application.setTestWebhookService(secretStore, dispatcher, nil, registry)

	fakeRuntime := &capturingRuntime{events: make(chan runtimeprotocol.Event, 1)}
	application.pluginStack.Dispatcher.Register("repo-watcher", fakeRuntime, []string{"webhook.received"}, nil, 1)

	if err := application.platform.Secrets.Set(context.Background(), "webhook.github.secret", []byte("fixture-webhook-secret")); err != nil {
		t.Fatalf("set webhook secret: %v", err)
	}

	if _, err := application.executeLocalAction(context.Background(), "repo-watcher", "req_webhook_register", runtimeaction.Action{
		Kind:                   "event.expose_webhook",
		WebhookRoute:           "github",
		WebhookMethods:         []string{"POST"},
		WebhookAuthStrategy:    "hmac_sha256",
		WebhookHeader:          "X-Hub-Signature-256",
		WebhookSecretRef:       "webhook.github.secret",
		WebhookSignaturePrefix: "sha256=",
		WebhookReplayProtection: &runtimeaction.WebhookReplayProtection{
			TimestampHeader:  "X-Raylea-Timestamp",
			EventIDHeader:    "X-Raylea-Event-Id",
			ToleranceSeconds: 300,
			Enforce:          true,
		},
	}); err != nil {
		t.Fatalf("register webhook action: %v", err)
	}

	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", application.handlePluginWebhook())
	server := httptest.NewServer(router)
	defer server.Close()

	body := []byte(`{"action":"` + strings.Repeat("a", 2*1024*1024) + `"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	eventID := "gh-evt-oversized-1"
	mac := hmac.New(sha256.New, []byte("fixture-webhook-secret"))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(eventID))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/webhooks/repo-watcher/github", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Raylea-Timestamp", timestamp)
	request.Header.Set("X-Raylea-Event-Id", eventID)
	request.Header.Set("X-Hub-Signature-256", signature)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform webhook request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected webhook status: got %d want %d", response.StatusCode, http.StatusBadRequest)
	}

	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode webhook error response: %v", err)
	}
	errorEnvelope, ok := payload["error"].(map[string]any)
	if !ok || errorEnvelope["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected webhook error payload: %#v", payload)
	}

	select {
	case event := <-fakeRuntime.events:
		t.Fatalf("unexpected webhook event dispatch: %#v", event)
	default:
	}
}
