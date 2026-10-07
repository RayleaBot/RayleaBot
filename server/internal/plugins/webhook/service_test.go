package webhook

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/go-chi/chi/v5"
)

func TestHandleWebhookEnsuresRuntime(t *testing.T) {
	t.Parallel()

	dispatcher := dispatch.New(nil, nil, nil, 16)
	events := make(chan chatevent.Event, 1)
	ensurer := &recordingRuntimeEnsurer{
		dispatcher: dispatcher,
		events:     events,
	}
	pluginCatalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "repo-watcher",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		Webhooks:          []plugins.WebhookScope{{ID: "github", Route: "github"}},
	}})
	registry := NewRegistry()
	registry.SyncSnapshots(pluginCatalog.List())

	service, err := New(Deps{
		Registry:   registry,
		Plugins:    pluginCatalog,
		Dispatcher: dispatcher,
		Runtime:    ensurer,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	router := chi.NewRouter()
	router.Post("/api/webhooks/{plugin_id}/{route}", service.HandleWebhook())
	server := httptest.NewServer(router)
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/webhooks/repo-watcher/github", bytes.NewReader([]byte(`{"ok":true}`)))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform webhook request: %v", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	if !ensurer.called {
		t.Fatal("expected runtime ensurer to be called")
	}

	select {
	case event := <-events:
		if event.EventType != "webhook.received" {
			t.Fatalf("event_type = %q, want webhook.received", event.EventType)
		}
		if event.Webhook == nil || event.Webhook.Route != "github" || event.Webhook.ReceivedAt <= 0 {
			t.Fatalf("webhook metadata = %#v, want typed github metadata", event.Webhook)
		}
		if _, exists := event.PayloadFields["webhook"]; exists {
			t.Fatalf("webhook metadata leaked into payload fields: %#v", event.PayloadFields)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected webhook event")
	}
}

func TestNewRequiresRuntimeDependencies(t *testing.T) {
	t.Parallel()

	if _, err := New(Deps{}); err == nil {
		t.Fatal("New accepted missing runtime dependencies")
	}
}

type recordingRuntimeEnsurer struct {
	dispatcher *dispatch.Dispatcher
	events     chan chatevent.Event
	called     bool
}

func (r *recordingRuntimeEnsurer) EnsurePluginRunning(_ context.Context, pluginID string) error {
	r.called = true
	r.dispatcher.Register(pluginID, &webhookRuntime{events: r.events}, []string{"webhook.received"}, nil, 1)
	return nil
}

type webhookRuntime struct {
	events chan chatevent.Event
}

func (r *webhookRuntime) DeliverEvent(_ context.Context, event chatevent.Event) (plugins.Delivery, error) {
	r.events <- event
	return plugins.Delivery{RequestID: "evt_webhook", Result: map[string]any{}}, nil
}

func (r *webhookRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (r *webhookRuntime) ReadyForEvents() bool {
	return r.Snapshot().State == pluginruntime.StateRunning
}
