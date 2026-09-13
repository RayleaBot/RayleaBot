package actions_test

import (
	"context"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

func TestSessionActionsRequireParentAndRespectProcessOwnership(t *testing.T) {
	r := conversation.New(conversation.Options{})
	t.Cleanup(r.Close)
	service := actions.New(actions.Deps{Conversations: r})
	event := chatevent.Event{EventID: "initial", BotID: "bot", SourceProtocol: "onebot11", SourceAdapter: "adapter", EventType: "message.group", Actor: &chatevent.Actor{ID: "actor"}, Target: &chatevent.Target{Type: "group", ID: "group"}}
	if _, err := service.Execute(t.Context(), "p", "action", plugins.Action{Kind: "session.wait"}, event); err == nil {
		t.Fatal("unowned parent accepted")
	}
	done := make(chan struct{})
	ctx := plugins.WithParentRequestID(plugins.WithRuntimeDone(t.Context(), done), "parent")
	result, err := service.Execute(ctx, "p", "action", plugins.Action{Kind: "session.wait"}, event)
	if err != nil {
		t.Fatal(err)
	}
	id := result["session_id"].(string)
	if result["scope"] != "user" || result["expires_at_ms"] == nil {
		t.Fatalf("invalid wait result: %#v", result)
	}
	finish := plugins.Action{Kind: "session.finish", SessionID: id}
	other := plugins.WithParentRequestID(plugins.WithRuntimeDone(t.Context(), make(chan struct{})), "other")
	if result, err := service.Execute(other, "other", "finish", finish, chatevent.Event{EventType: "scheduler.trigger"}); err != nil || result["finished"] != false {
		t.Fatal("foreign process finished session")
	}
	for _, want := range []bool{true, false} {
		if result, err := service.Execute(ctx, "p", "finish", finish, chatevent.Event{EventType: "scheduler.trigger"}); err != nil || result["finished"] != want {
			t.Fatalf("finish=%#v err=%v", result, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.Execute(canceled, "p", "late", plugins.Action{Kind: "session.wait"}, event); err == nil {
		t.Fatal("ended parent registered a session")
	}
}
