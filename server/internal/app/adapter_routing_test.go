package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type routingPermissions struct{}

func (routingPermissions) PermissionDeclared(context.Context, string, string) bool      { return true }
func (routingPermissions) PermissionPlatforms(context.Context, string, string) []string { return nil }
func (routingPermissions) ListPluginSnapshots() []plugins.Snapshot                      { return nil }

func oneBotRoutingEndpoint(t *testing.T, name string) (config.AdapterInstance, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request onebot11.APICallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode API request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, present := request.Params["source_adapter"]; present {
			t.Error("host source_adapter selector reached provider")
		}
		if _, present := request.Params["source_protocol"]; present {
			t.Error("host source_protocol selector reached provider")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "retcode": 0,
			"data": map[string]any{"group_name": name + " group", "nickname": name + " user", "role": "member"},
		})
	}))
	t.Cleanup(endpoint.Close)
	return config.AdapterInstance{
		ID: name, Type: config.AdapterTypeOneBot11, Enabled: true,
		OneBot11: &config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: endpoint.URL}},
	}, &calls
}

func TestOneBotActionsUseConfiguredInstanceAndParent(t *testing.T) {
	t.Parallel()
	parent := chatevent.Event{SourceProtocol: "onebot11", SourceAdapter: "second"}
	for _, tc := range []struct {
		name   string
		data   string
		parent chatevent.Event
		change func(*config.Config)
		want   string
	}{
		{name: "parent owns second instance", parent: parent, want: "second"},
		{name: "matching explicit selector", parent: parent, data: `{"source_adapter":"second","source_protocol":"onebot11","group_id":"200"}`, want: "second"},
		{name: "platform explicit selector", parent: chatevent.Event{SourceProtocol: "scheduler", SourceAdapter: "scheduler.internal"}, data: `{"source_adapter":"second","group_id":"200"}`, want: "second"},
		{name: "parent selector conflict", parent: parent, data: `{"source_adapter":"first","group_id":"200"}`},
		{name: "no context is ambiguous"},
		{name: "protocol only is ambiguous", data: `{"source_protocol":"onebot11","group_id":"200"}`},
		{name: "unknown instance", data: `{"source_adapter":"unknown","group_id":"200"}`},
		{name: "disabled instance", parent: parent, change: func(cfg *config.Config) { cfg.Adapters[1].Enabled = false }},
		{name: "removed instance", parent: parent, change: func(cfg *config.Config) { cfg.Adapters = cfg.Adapters[:1] }},
		{name: "changed instance protocol", parent: parent, change: func(cfg *config.Config) { cfg.Adapters[1].Type = "qqofficial" }},
		{name: "reordered instances", parent: parent, want: "second", change: func(cfg *config.Config) { cfg.Adapters[0], cfg.Adapters[1] = cfg.Adapters[1], cfg.Adapters[0] }},
		{name: "one enabled candidate", want: "first", change: func(cfg *config.Config) { cfg.Adapters[1].Enabled = false }},
		{name: "no adapters", change: func(cfg *config.Config) { cfg.Adapters = nil }},
		{name: "QQ only", change: func(cfg *config.Config) {
			cfg.Adapters = []config.AdapterInstance{{ID: "qq", Type: "qqofficial", Enabled: true}}
		}},
		{name: "QQ parent conflicts with OneBot selector", parent: chatevent.Event{SourceProtocol: "qqofficial", SourceAdapter: "qq"}, data: `{"source_adapter":"second","group_id":"200"}`},
		{name: "parent missing instance", parent: chatevent.Event{SourceProtocol: "onebot11"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, firstCalls := oneBotRoutingEndpoint(t, "first")
			second, secondCalls := oneBotRoutingEndpoint(t, "second")
			cfg := config.Config{Adapters: []config.AdapterInstance{first, second}}
			state := buildEvents(eventDeps{Config: cfg, CurrentConfig: func() config.Config { return cfg }, Logger: discardLogger()})
			t.Cleanup(state.Close)
			if tc.change != nil {
				tc.change(&cfg)
			}
			data := tc.data
			if data == "" {
				data = `{"group_id":"200"}`
			}
			action, err := pluginruntime.ParseLocalAction("group.info.get", json.RawMessage(data))
			if err != nil {
				t.Fatal(err)
			}
			service := actions.New(actions.Deps{Permissions: routingPermissions{}, ResolveOneBotAdapter: state.ResolveOneBotAdapter})
			result, err := service.Execute(t.Context(), "fixture", "action", *action, tc.parent)
			if tc.want == "" {
				var coded *plugins.Error
				if !errors.As(err, &coded) || coded.Code != "plugin.protocol_violation" {
					t.Fatalf("error = %v, want plugin.protocol_violation", err)
				}
			} else if err != nil || result["group_name"] != tc.want+" group" {
				t.Fatalf("result = %#v, error = %v", result, err)
			}
			wantFirst, wantSecond := int32(0), int32(0)
			if tc.want == "first" {
				wantFirst = 1
			}
			if tc.want == "second" {
				wantSecond = 1
			}
			if firstCalls.Load() != wantFirst || secondCalls.Load() != wantSecond {
				t.Fatalf("API calls = first:%d second:%d, want first:%d second:%d", firstCalls.Load(), secondCalls.Load(), wantFirst, wantSecond)
			}
		})
	}
}

func TestMetadataEnrichmentIsolatedByAdapterInstance(t *testing.T) {
	t.Parallel()
	first, firstCalls := oneBotRoutingEndpoint(t, "first")
	second, secondCalls := oneBotRoutingEndpoint(t, "second")
	cfg := config.Config{Adapters: []config.AdapterInstance{first, second}}
	state := buildEvents(eventDeps{Config: cfg, CurrentConfig: func() config.Config { return cfg }, Logger: discardLogger()})
	t.Cleanup(state.Close)
	event := chatevent.NormalizedEvent{
		SourceProtocol: "onebot11", EventType: "message.group", ConversationType: "group",
		ConversationID: "200", SenderID: "300", PayloadFields: map[string]any{"sender": map[string]any{}},
	}
	for i := 0; i < 2; i++ {
		for _, id := range []string{"second", "first"} {
			event.SourceAdapter = id
			beforeFirst, beforeSecond := firstCalls.Load(), secondCalls.Load()
			enriched := state.EnrichEventMetadata(t.Context(), event)
			if enriched.TargetName != id+" group" || enriched.ActorNickname != id+" user" {
				t.Fatalf("%s metadata = %#v", id, enriched)
			}
			if id == "first" && secondCalls.Load() != beforeSecond || id == "second" && firstCalls.Load() != beforeFirst {
				t.Fatal("metadata lookup reached another instance")
			}
		}
		cfg.Adapters[0], cfg.Adapters[1] = cfg.Adapters[1], cfg.Adapters[0]
	}
	if firstCalls.Load() != 2 || secondCalls.Load() != 2 {
		t.Fatalf("caches not reused independently: first:%d second:%d", firstCalls.Load(), secondCalls.Load())
	}
	for _, id := range []string{"", "unknown"} {
		event.SourceAdapter = id
		if got := state.EnrichEventMetadata(t.Context(), event); got.TargetName != "" || got.ActorNickname != "" {
			t.Fatalf("unscoped metadata = %#v", got)
		}
	}
	event.SourceAdapter = "second"
	cfg.Adapters[1].Enabled = false
	if got := state.EnrichEventMetadata(t.Context(), event); got.TargetName != "" {
		t.Fatal("disabled instance supplied cached metadata")
	}
	cfg.Adapters = cfg.Adapters[:1]
	if got := state.EnrichEventMetadata(t.Context(), event); got.TargetName != "" {
		t.Fatal("removed instance supplied cached metadata")
	}
	if firstCalls.Load() != 2 || secondCalls.Load() != 2 {
		t.Fatal("unavailable route issued metadata queries")
	}
}

func TestOneBotActionsWithoutConfiguredOneBot(t *testing.T) {
	t.Parallel()
	for _, cfg := range []config.Config{{}, {Adapters: []config.AdapterInstance{{ID: "qq", Type: "qqofficial", Enabled: true, QQOfficial: &config.QQOfficialConfig{}}}}} {
		state := buildEvents(eventDeps{Config: cfg, Logger: discardLogger()})
		t.Cleanup(state.Close)
		service := actions.New(actions.Deps{Permissions: routingPermissions{}, ResolveOneBotAdapter: state.ResolveOneBotAdapter})
		_, err := service.Execute(t.Context(), "fixture", "action", plugins.Action{Kind: "group.list"}, chatevent.Event{})
		var coded *plugins.Error
		if !errors.As(err, &coded) || coded.Code != "plugin.protocol_violation" {
			t.Fatalf("missing capability returned %v", err)
		}
	}
}

func TestOneBotProviderActionUsesSelectedInstanceProvider(t *testing.T) {
	t.Parallel()
	var instances []config.AdapterInstance
	var actionCalls [2]atomic.Int32
	for index, provider := range []string{"LLOneBot", "NapCat.Onebot"} {
		id := []string{"first", "second"}[index]
		endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request onebot11.APICallRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode provider request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var data any = map[string]any{}
			switch request.Action {
			case "get_version_info":
				data = map[string]any{"app_name": provider, "protocol_version": "v11"}
			case "get_login_info":
				data = map[string]any{"user_id": "100", "nickname": id}
			case "set_group_sign":
				actionCalls[index].Add(1)
				if index != 1 || request.Params["group_id"] != "200" {
					t.Errorf("NapCat action reached %s: %#v", id, request)
				}
			case "get_grouped_friend_list":
				actionCalls[index].Add(1)
				if index != 0 {
					t.Errorf("LuckyLillia action reached %s", id)
				}
				data = []any{}
			default:
				actionCalls[index].Add(1)
				t.Errorf("unexpected provider action: %s", request.Action)
			}
			if _, present := request.Params["source_adapter"]; present {
				t.Error("host selector reached provider")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "retcode": 0, "data": data})
		}))
		t.Cleanup(endpoint.Close)
		instances = append(instances, config.AdapterInstance{
			ID: id, Type: "onebot11", Enabled: true,
			OneBot11: &config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: endpoint.URL}},
		})
	}
	state := buildEvents(eventDeps{Config: config.Config{Adapters: instances}, Logger: discardLogger()})
	t.Cleanup(state.Close)
	for index, id := range []string{"first", "second"} {
		shell := state.OneBotShells[id]
		providerReady := make(chan struct{}, 1)
		wantProvider := []string{"luckylillia", "napcat"}[index]
		shell.SetStateHandler(func(snapshot onebot11.Snapshot) {
			if snapshot.DetectedProvider() == wantProvider {
				select {
				case providerReady <- struct{}{}:
				default:
				}
			}
		})
		shell.Start(t.Context())
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := shell.Stop(ctx); err != nil {
				t.Errorf("stop %s: %v", id, err)
			}
		})
		select {
		case <-providerReady:
		case <-time.After(3 * time.Second):
			t.Fatalf("provider identity was not discovered for %s", id)
		}
	}
	service := actions.New(actions.Deps{Permissions: routingPermissions{}, ResolveOneBotAdapter: state.ResolveOneBotAdapter})
	for _, tc := range []struct {
		kind    string
		adapter string
		want    int
	}{
		{kind: "provider.napcat.group.sign.set", adapter: "second", want: 1},
		{kind: "provider.napcat.group.sign.set", adapter: "first", want: -1},
		{kind: "provider.luckylillia.friend_groups.get", adapter: "first", want: 0},
		{kind: "provider.luckylillia.friend_groups.get", adapter: "second", want: -1},
	} {
		data, err := json.Marshal(map[string]any{"source_adapter": tc.adapter, "source_protocol": "onebot11", "group_id": "200"})
		if err != nil {
			t.Fatal(err)
		}
		action, err := pluginruntime.ParseLocalAction(tc.kind, data)
		if err != nil {
			t.Fatal(err)
		}
		before := [2]int32{actionCalls[0].Load(), actionCalls[1].Load()}
		_, err = service.Execute(t.Context(), "fixture", "provider-action", *action, chatevent.Event{})
		if tc.want < 0 {
			var coded *plugins.Error
			if !errors.As(err, &coded) || coded.Code != "adapter.provider_extension_not_supported" {
				t.Fatalf("%s on %s returned %v, want provider rejection", tc.kind, tc.adapter, err)
			}
		} else if err != nil {
			t.Fatalf("%s on %s: %v", tc.kind, tc.adapter, err)
		}
		for index := range actionCalls {
			want := before[index]
			if index == tc.want {
				want++
			}
			if got := actionCalls[index].Load(); got != want {
				t.Fatalf("%s on %s: instance %d received %d calls, want %d", tc.kind, tc.adapter, index, got, want)
			}
		}
	}
}
