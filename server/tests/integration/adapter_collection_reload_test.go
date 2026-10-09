package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

type collectionConfig struct{}

func (collectionConfig) CurrentConfig() config.Config { return config.Config{} }

func TestAdapterCollectionReloadKeepsIngressAndOutboundTogether(t *testing.T) {
	registry := adapterservice.NewRegistry(config.Config{}, nil, nil)
	received := make(chan chatevent.NormalizedEvent, 8)
	service, err := adapterservice.NewService(collectionConfig{}, adapterservice.Instances{
		Registry: registry,
		NewOneBot11: func(id string, settings config.OneBotConfig, adapter config.AdapterConfig) *onebot11.Shell {
			shell := onebot11.NewForTest(id, settings, adapter, nil, true)
			shell.SetEventHandler(func(_ context.Context, event chatevent.NormalizedEvent) { received <- event })
			return shell
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := service.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	router := outbound.NewLiveRouter(func() outbound.Routes {
		snapshot := registry.Snapshot()
		cfg := snapshot.Config()
		routes := outbound.Routes{Config: &cfg, Senders: map[string]outbound.ActionSender{}, Protocols: map[string]string{}}
		for _, instance := range cfg.Adapters {
			if shell := snapshot.OneBot11(instance.ID); shell != nil {
				routes.Senders[instance.ID], routes.Protocols[instance.ID] = shell, instance.Type
			}
		}
		return routes
	})
	mux := chi.NewRouter()
	management.NewProtocolHandlers(service, "127.0.0.1").RegisterPublicRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	settings := config.OneBotConfig{
		ReverseWS: config.OneBotTransportConfig{Enabled: true, URL: "ws://127.0.0.1/fixture"},
		Webhook:   config.OneBotTransportConfig{Enabled: true, URL: "http://127.0.0.1/fixture"},
	}
	instance := func(id string) config.AdapterInstance {
		return config.AdapterInstance{ID: id, Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &settings}
	}
	apply := func(instances ...config.AdapterInstance) {
		t.Helper()
		if err := service.ApplyConfigReload(config.Config{Adapters: instances}); err != nil {
			t.Fatal(err)
		}
	}
	send := func(id string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		result, err := router.SendMessage(ctx, chatevent.OutboundMessageSend{
			SourceAdapter: id, TargetType: "private", TargetID: "200",
			Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "fixture"}}},
		})
		if err != nil || result.SourceAdapter != id || result.MessageID != "500" {
			t.Fatalf("send via %s: %+v, %v", id, result, err)
		}
	}
	webhook := func(id string, accepted bool) {
		t.Helper()
		payload := `{"post_type":"message","message_type":"private","self_id":100,"user_id":200,"message_id":123,"time":1700000000,"message":[{"type":"text","data":{"text":"fixture"}}]}`
		response, err := http.Post(server.URL+"/api/adapters/"+id+"/webhook", "application/json", bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if (response.StatusCode == http.StatusAccepted) != accepted {
			t.Fatalf("webhook %s status=%d", id, response.StatusCode)
		}
		if accepted {
			select {
			case event := <-received:
				if event.SourceAdapter != id {
					t.Fatalf("event source=%s, want %s", event.SourceAdapter, id)
				}
			case <-time.After(time.Second):
				t.Fatalf("no event from %s", id)
			}
		}
	}
	assertRemoved := func(id string, old *onebot11.Shell, disconnected <-chan struct{}) {
		t.Helper()
		if _, ok := service.OneBot11Ingress(id); ok {
			t.Fatalf("removed ingress %s survived", id)
		}
		if _, err := router.ResolveAdapterID(id, "onebot11"); err == nil {
			t.Fatalf("removed route %s survived", id)
		}
		if old.Snapshot().State != onebot11.StateStopped {
			t.Fatalf("removed %s did not stop", id)
		}
		if _, err := old.CallAPIAny(t.Context(), "send_msg", nil); err == nil {
			t.Fatal("retired sender accepted an API call")
		}
		if err := old.AcceptWebhookPayload(t.Context(), []byte(`{}`)); err == nil {
			t.Fatal("retired ingress accepted a webhook")
		}
		select {
		case <-disconnected:
		case <-time.After(time.Second):
			t.Fatalf("removed %s still connected", id)
		}
		webhook(id, false)
	}
	apply(instance("steady"))
	steady := registry.Snapshot().OneBot11("steady")
	dialReloadPeer(t, server.URL, "steady")
	send("steady")
	// Readers race against membership publication and must always get a complete table.
	stopReaders := make(chan struct{})
	var readers sync.WaitGroup
	readers.Go(func() {
		for {
			select {
			case <-stopReaders:
				return
			default:
			}
			snapshot := registry.Snapshot()
			for _, item := range snapshot.Config().Adapters {
				if item.Type == "onebot11" && snapshot.OneBot11(item.ID) == nil {
					t.Error("partially published OneBot collection")
					return
				}
				if item.Type == "qqofficial" && snapshot.QQOfficial(item.ID) == nil {
					t.Error("partially published QQ collection")
					return
				}
			}
			service.Adapters()
			service.OneBot11Ingress("added")
			if id, err := router.ResolveAdapterID("steady", "onebot11"); err != nil || id != "steady" {
				t.Errorf("unchanged route = %q, %v", id, err)
				return
			}
		}
	})
	defer func() { close(stopReaders); readers.Wait() }()
	apply(instance("steady"), instance("added"))
	added := registry.Snapshot().OneBot11("added")
	addedGone := dialReloadPeer(t, server.URL, "added")
	webhook("added", true)
	send("added")
	apply(instance("added"), instance("steady"))
	send("steady")
	apply(instance("steady"))
	assertRemoved("added", added, addedGone)
	apply(instance("steady"), instance("old-name"))
	old := registry.Snapshot().OneBot11("old-name")
	oldGone := dialReloadPeer(t, server.URL, "old-name")
	apply(instance("steady"), instance("new-name"))
	assertRemoved("old-name", old, oldGone)
	newShell := registry.Snapshot().OneBot11("new-name")
	newGone := dialReloadPeer(t, server.URL, "new-name")
	webhook("new-name", true)
	send("new-name")
	apply(instance("steady"), config.AdapterInstance{ID: "new-name", Type: "qqofficial", QQOfficial: &config.QQOfficialConfig{}})
	assertRemoved("new-name", newShell, newGone)
	if registry.Snapshot().QQOfficial("new-name") == nil {
		t.Fatal("replacement QQ runtime missing")
	}
	if registry.Snapshot().OneBot11("steady") != steady {
		t.Fatal("unchanged runtime was replaced")
	}
	send("steady")
	if got := service.AdapterStates(); len(got) != 2 || !slices.ContainsFunc(got, func(a adapterservice.Status) bool {
		return a.ID == "new-name" && a.Protocol == "qqofficial" && !a.Enabled
	}) {
		t.Fatalf("states=%+v", got)
	}
}

func dialReloadPeer(t *testing.T, baseURL, id string) <-chan struct{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(baseURL, "http")+"/api/adapters/"+id+"/reverse-ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() { _ = conn.CloseNow(); <-done })
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"post_type":"meta_event","meta_event_type":"lifecycle","sub_type":"enable","self_id":100}`)); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(done)
		defer func() { _ = conn.CloseNow() }()
		for {
			_, data, err := conn.Read(t.Context())
			if err != nil {
				return
			}
			var request struct {
				Echo json.RawMessage `json:"echo"`
			}
			if json.Unmarshal(data, &request) != nil {
				return
			}
			reply := fmt.Sprintf(`{"status":"ok","retcode":0,"echo":%s,"data":{"message_id":500}}`, request.Echo)
			if err := conn.Write(t.Context(), websocket.MessageText, []byte(reply)); err != nil {
				return
			}
		}
	}()
	// A successful peer ping is ordered after the lifecycle frame at the server.
	if err := conn.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	return done
}
