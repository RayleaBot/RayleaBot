package webhook

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/go-chi/chi/v5"
)

func TestHandleWebhookEnsuresRuntimeWithoutBotID(t *testing.T) {
	t.Parallel()

	dispatcher := dispatch.New(nil, nil, nil, 16)
	events := make(chan runtimeprotocol.Event, 1)
	ensurer := &recordingRuntimeEnsurer{
		dispatcher: dispatcher,
		events:     events,
	}
	registry := NewRegistry()
	registry.Register(Registration{
		PluginID:     "repo-watcher",
		Route:        "github",
		Methods:      []string{http.MethodPost},
		AuthStrategy: "fixed_token",
		Header:       "X-Webhook-Token",
		SecretRef:    "webhook.github.secret",
	})

	service := New(Deps{
		Registry: registry,
		Secrets:  &staticSecretStore{values: map[string][]byte{"webhook.github.secret": []byte("fixture-token")}},
		Plugins: plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "repo-watcher",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
		}}),
		Dispatcher:   dispatcher,
		Runtime:      ensurer,
		Capabilities: alwaysCapabilityView{},
	})

	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", service.HandleWebhook())
	server := httptest.NewServer(router)
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/webhooks/repo-watcher/github", bytes.NewReader([]byte(`{"ok":true}`)))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}
	request.Header.Set("X-Webhook-Token", "fixture-token")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform webhook request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	if !ensurer.called {
		t.Fatal("expected runtime ensurer to be called")
	}
	if ensurer.botID != "" {
		t.Fatalf("botID = %q, want empty", ensurer.botID)
	}

	select {
	case event := <-events:
		if event.EventType != "webhook.received" {
			t.Fatalf("event_type = %q, want webhook.received", event.EventType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected webhook event")
	}
}

type recordingRuntimeEnsurer struct {
	dispatcher *dispatch.Dispatcher
	events     chan runtimeprotocol.Event
	called     bool
	botID      string
}

func (r *recordingRuntimeEnsurer) CurrentBotID() string {
	return ""
}

func (r *recordingRuntimeEnsurer) EnsurePluginRunning(_ context.Context, pluginID string, botID string) error {
	r.called = true
	r.botID = botID
	r.dispatcher.Register(pluginID, &webhookRuntime{events: r.events}, []string{"webhook.received"}, nil, 1)
	return nil
}

type webhookRuntime struct {
	events chan runtimeprotocol.Event
}

func (r *webhookRuntime) DeliverEvent(_ context.Context, event runtimeprotocol.Event) (runtimemanager.Delivery, error) {
	r.events <- event
	return runtimemanager.Delivery{RequestID: "evt_webhook", Result: map[string]any{}}, nil
}

func (r *webhookRuntime) Snapshot() runtimemanager.Snapshot {
	return runtimemanager.Snapshot{State: runtimemanager.StateRunning}
}

type alwaysCapabilityView struct{}

func (alwaysCapabilityView) CapabilityDeclared(context.Context, string, string) bool {
	return true
}

func (alwaysCapabilityView) WebhookParameters(context.Context, string, string) (plugins.WebhookScope, bool) {
	return plugins.WebhookScope{}, true
}

type staticSecretStore struct {
	values map[string][]byte
}

func (s *staticSecretStore) Get(_ context.Context, key string) ([]byte, error) {
	return s.values[key], nil
}

func (s *staticSecretStore) Set(context.Context, string, []byte) error {
	return nil
}

func (s *staticSecretStore) Delete(context.Context, string) error {
	return nil
}

func (s *staticSecretStore) List(context.Context) ([]string, error) {
	return nil, nil
}
