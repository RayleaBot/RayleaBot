package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/secrets"
	"rayleabot/server/internal/storage"
)

type capturingRuntime struct {
	events chan runtime.Event
}

func (r *capturingRuntime) DeliverEvent(_ context.Context, event runtime.Event) (runtime.Delivery, error) {
	select {
	case r.events <- event:
	default:
	}
	return runtime.Delivery{
		RequestID: "event_webhook_1",
		Result:    map[string]any{},
	}, nil
}

func (r *capturingRuntime) Snapshot() runtime.Snapshot {
	return runtime.Snapshot{State: runtime.StateRunning}
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

	application := &App{
		Config: config.Config{
			Server: config.ServerConfig{
				Host: "127.0.0.1",
				Port: 8080,
			},
			Auth: config.AuthConfig{
				AutoGrantCapabilities: []string{"event.expose_webhook", "event.raw_payload"},
			},
		},
		Logger:    slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		Secrets:   secretStore,
		Plugins:   plugins.NewCatalog([]plugins.Snapshot{{PluginID: "repo-watcher", Name: "Repo Watcher", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running"}}),
		Dispatcher: dispatch.New(slog.Default(), nil, nil, 16),
		Runtimes:   newRuntimeRegistry(slog.Default(), runtime.Options{}),
		webhooks:   newPluginWebhookRegistry(),
		grantRepository: &stubLifecycleGrantRepository{
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
		},
	}
	application.pluginLifecycle = newPluginLifecycleController(application)

	fakeRuntime := &capturingRuntime{events: make(chan runtime.Event, 1)}
	application.Dispatcher.Register("repo-watcher", fakeRuntime, []string{"webhook.received"}, nil)

	if err := application.Secrets.Set(context.Background(), "webhook.github.secret", []byte("fixture-webhook-secret")); err != nil {
		t.Fatalf("set webhook secret: %v", err)
	}

	if _, err := application.executeLocalAction(context.Background(), "repo-watcher", "req_webhook_register", runtime.Action{
		Kind:                   "event.expose_webhook",
		WebhookRoute:           "github",
		WebhookMethods:         []string{"POST"},
		WebhookAuthStrategy:    "hmac_sha256",
		WebhookHeader:          "X-Hub-Signature-256",
		WebhookSecretRef:       "webhook.github.secret",
		WebhookSignaturePrefix: "sha256=",
	}); err != nil {
		t.Fatalf("register webhook action: %v", err)
	}

	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", application.handlePluginWebhook())
	server := httptest.NewServer(router)
	defer server.Close()

	body := []byte(`{"action":"opened"}`)
	mac := hmac.New(sha256.New, []byte("fixture-webhook-secret"))
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/webhooks/repo-watcher/github", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
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
