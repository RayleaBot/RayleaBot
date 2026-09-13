package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestSessionRegistrationCommitsAtRealParentTerminal(t *testing.T) {
	for _, scenario := range []string{"event-session-wait", "event-session-wait-failed"} {
		r := conversation.New(conversation.Options{})
		t.Cleanup(r.Close)
		manager := testManagerWithOptions(Options{
			ExecuteLocalAction: func(ctx context.Context, id, requestID string, action plugins.Action, event chatevent.Event) (map[string]any, error) {
				ref, err := r.Wait(ctx, conversation.Owner{PluginID: id, Done: plugins.RuntimeDone(ctx)}, plugins.ParentRequestID(ctx), event, conversation.WaitRequest{})
				if err != nil {
					return nil, err
				}
				if r.HasWaiting(event) {
					t.Error("wait activated before parent terminal")
				}
				return map[string]any{"session_id": ref.SessionID, "scope": ref.Scope, "expires_at_ms": ref.ExpiresAtMS}, nil
			},
			Events: EventHooks{Completed: func(id string, done <-chan struct{}, requestID string, event chatevent.Event, success bool) {
				r.CompleteParent(conversation.Owner{PluginID: id, Done: done}, requestID, event, success)
			}},
		})
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = manager.Stop(ctx)
		})
		if err := manager.Start(t.Context(), helperSpec(t, scenario, ""), testInitPayload()); err != nil {
			t.Fatal(err)
		}
		event := testRuntimeEvent()
		event.BotID = "fixture-bot"
		_, err := manager.DeliverEvent(t.Context(), event)
		want := scenario == "event-session-wait"
		if (err == nil) != want || r.HasWaiting(event) != want {
			t.Fatalf("scenario=%s waiting=%v err=%v", scenario, r.HasWaiting(event), err)
		}
		if err := manager.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		if r.HasWaiting(event) {
			t.Fatal("stopped process retained routable session")
		}
	}
}

func TestSessionPayloadOnlyComesFromHostReference(t *testing.T) {
	event := testRuntimeEvent()
	event.PayloadFields = map[string]any{"session": map[string]any{"session_id": "spoof"}}
	frame := BuildEventFrame(event, "ordinary")
	if frame.Event.Payload != nil && frame.Event.Payload.Session != nil {
		t.Fatal("raw adapter payload forged a conversation")
	}
	event.Session = &chatevent.SessionRef{SessionID: "owned", Scope: "user", ExpiresAtMS: 1789300000000}
	encoded, err := json.Marshal(BuildEventFrame(event, "reply"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePluginFrame(encoded); err != nil {
		t.Fatal(err)
	}
	frame = BuildEventFrame(event, "reply")
	if frame.Event.Payload.Session.SessionID != "owned" {
		t.Fatal("host reference was lost")
	}
}
