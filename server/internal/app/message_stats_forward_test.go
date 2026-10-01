package app

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"testing"
)

type statsForwardAdapter struct{ err error }

func (a statsForwardAdapter) CallAPIAny(context.Context, string, map[string]any) (any, error) {
	return map[string]any{"message_id": "forward"}, a.err
}
func (statsForwardAdapter) DetectedProvider() string { return "napcat" }

func TestForwardCountsWholeSendOnlyAfterPlatformConfirmation(t *testing.T) {
	for _, failure := range []error{nil, context.DeadlineExceeded, &chatevent.SendError{Code: errorcodes.AdapterSendUnconfirmed}} {
		count := 0
		adapter := observedOneBotAdapter{OneBotAdapter: statsForwardAdapter{failure}, id: "bot", messageSent: func(id, protocol string) {
			if id != "bot" || protocol != "onebot11" {
				t.Fatal("wrong forward attribution")
			}
			count++
		}}
		for _, action := range []string{"send_group_forward_msg", "send_private_forward_msg", "get_forward_msg", "get_group_msg_history"} {
			_, _ = adapter.CallAPIAny(t.Context(), action, map[string]any{"messages": []any{1, 2, 3}})
		}
		want := 0
		if failure == nil {
			want = 2
		}
		if count != want {
			t.Fatalf("count=%d want=%d", count, want)
		}
	}
}

// Keep this assertion with the assembly wrapper: local actions must retain the
// same OneBot API surface, including provider discovery.
var _ actions.OneBotAdapter = observedOneBotAdapter{}

func TestLocalForwardActionUsesResolvedInstanceCounter(t *testing.T) {
	instance, calls := oneBotRoutingEndpoint(t, "forward-bot")
	count := 0
	state := buildEvents(eventDeps{Config: config.Config{Adapters: []config.AdapterInstance{instance}}, Logger: discardLogger(), MessageSent: func(id, protocol string) {
		if id != "forward-bot" || protocol != "onebot11" {
			t.Fatalf("wrong forward attribution %s %s", id, protocol)
		}
		count++
	}})
	t.Cleanup(state.Close)
	service := actions.New(actions.Deps{ResolveOneBotAdapter: state.ResolveOneBotAdapter})
	_, err := service.Execute(t.Context(), "fixture", "forward", plugins.Action{Kind: "message.forward.send", RawData: map[string]any{"target_type": "group", "target_id": "200", "messages": []any{map[string]any{"type": "node"}, map[string]any{"type": "node"}}}}, chatevent.Event{})
	if err != nil || count != 1 || calls.Load() != 1 {
		t.Fatalf("forward count=%d API calls=%d err=%v", count, calls.Load(), err)
	}
}
