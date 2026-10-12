package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

func servicePipe(t *testing.T) (*pipeRuntime, *Registry) {
	t.Helper()
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), nil)
	p.handle.Spec.PluginID = "provider"
	p.handle.Spec.Services = []plugins.Service{{Name: "records", Version: 1, Methods: []string{"list"}}}
	r := NewRegistry(nil, Options{})
	r.Replace("provider", p.manager)
	return p, r
}

func TestManagementAccountServiceOriginSurvivesDetach(t *testing.T) {
	for _, detached := range []bool{false, true} {
		name := "foreground"
		if detached {
			name = "detached"
		}
		t.Run(name, func(t *testing.T) {
			provider, registry := servicePipe(t)
			consumer := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), func(ctx context.Context, _, _ string, action plugins.Action, origin chatevent.Event) (map[string]any, error) {
				return registry.CallService(ctx, "consumer", *action.ServiceCall, origin)
			})
			bot := pluginwire.BotAccountRef{SourceProtocol: "qqofficial", BotID: "opaque:id"}
			done, frame := consumer.deliver(context.Background(), chatevent.Event{EventID: "manage", EventType: "management.action", SourceProtocol: "management", SourceAdapter: "management.ui", Timestamp: time.Now().Unix(), PayloadFields: map[string]any{"action": "list", "page_id": "records", "bot": bot, "payload": map[string]any{"bot": map[string]any{"bot_id": "spoof"}, "page_id": "spoof"}}})
			eventID := frame["request_id"].(string)
			payload := frame["event"].(map[string]any)["payload"].(map[string]any)
			if payload["page_id"] != "records" || payload["bot"].(map[string]any)["bot_id"] != bot.BotID || payload["payload"].(map[string]any)["page_id"] != "spoof" {
				t.Fatal(payload)
			}
			var outcome deliveryOutcome
			if detached {
				consumer.detach(eventID, "detach", map[string]any{})
				outcome = awaitDelivery(t, done)
				if outcome.err != nil {
					t.Fatal(outcome.err)
				}
			}
			consumer.send(map[string]any{"type": "action", "request_id": "call", "parent_request_id": eventID, "action": "plugin.call", "data": map[string]any{"target_plugin_id": "provider", "service": "records", "service_version": 1, "method": "list", "params": map[string]any{"origin": map[string]any{"bot": map[string]any{"bot_id": "spoof"}}}}})
			request := provider.next()
			service := request["event"].(map[string]any)["payload"].(map[string]any)["service_request"].(map[string]any)
			origin := service["origin"].(map[string]any)
			selected := origin["bot"].(map[string]any)
			if origin["source_protocol"] != "management" || origin["source_adapter"] != "management.ui" || selected["source_protocol"] != bot.SourceProtocol || selected["bot_id"] != bot.BotID {
				t.Fatal(origin)
			}
			provider.send(map[string]any{"type": "result", "request_id": request["request_id"], "status": "success", "data": map[string]any{}})
			if response := consumer.next(); response["request_id"] != "call" || response["type"] != "result" {
				t.Fatal(response)
			}
			consumer.send(map[string]any{"type": "result", "request_id": eventID, "status": "success", "data": map[string]any{}})
			if detached {
				if err := awaitDetachedEnd(t, outcome.delivery.Detached); err != nil {
					t.Fatal(err)
				}
			} else if result := awaitDelivery(t, done); result.err != nil {
				t.Fatal(result.err)
			}
		})
	}
}

func TestManagementAccountDoesNotChangeOtherServiceOrigins(t *testing.T) {
	provider, registry := servicePipe(t)
	for _, origin := range []chatevent.Event{
		{EventID: "chat", EventType: "message.group", SourceProtocol: "onebot11", SourceAdapter: "x", BotID: "10001", Actor: &chatevent.Actor{ID: "user"}, Target: &chatevent.Target{Type: "group", ID: "group"}},
		{EventID: "schedule", EventType: "scheduler.trigger", SourceProtocol: "scheduler", SourceAdapter: "scheduler.internal", PayloadFields: map[string]any{"task_id": "daily", "bot": pluginwire.BotAccountRef{SourceProtocol: "onebot11", BotID: "spoof"}}},
		{EventID: "global", EventType: "management.action", SourceProtocol: "management", SourceAdapter: "management.ui"},
	} {
		t.Run(origin.EventID, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := registry.CallService(t.Context(), "consumer", plugins.ServiceCall{TargetPluginID: "provider", Service: "records", ServiceVersion: 1, Method: "list", Params: map[string]any{}}, origin)
				done <- err
			}()
			frame := provider.next()
			service := frame["event"].(map[string]any)["payload"].(map[string]any)["service_request"].(map[string]any)
			projected := service["origin"].(map[string]any)
			if _, exists := projected["bot"]; exists {
				t.Fatal(projected)
			}
			if projected["source_protocol"] != origin.SourceProtocol || projected["source_adapter"] != origin.SourceAdapter {
				t.Fatal(projected)
			}
			if origin.EventID == "chat" && projected["bot_id"] != "10001" || origin.EventID == "schedule" && projected["task_id"] != "daily" {
				t.Fatal(projected)
			}
			provider.send(map[string]any{"type": "result", "request_id": frame["request_id"], "status": "success", "data": map[string]any{}})
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
