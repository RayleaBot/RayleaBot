package chatpolicy_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

type sessionObservation struct {
	event chatevent.Event
	err   error
}
type sessionRuntime struct {
	owner    conversation.Owner
	registry *conversation.Registry
	handle   func(context.Context, string, chatevent.Event) error
	observed chan sessionObservation
}

func (r *sessionRuntime) ReadyForEvents() bool         { return r.owner.Alive() }
func (r *sessionRuntime) ProcessDone() <-chan struct{} { return r.owner.Done }
func (r *sessionRuntime) DeliverEvent(ctx context.Context, event chatevent.Event) (plugins.Delivery, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	id := event.EventID + "-parent"
	var err error
	if event.Session != nil && !r.registry.BeginInput(r.owner, event) {
		err = fmt.Errorf("input expired")
	} else if r.handle != nil {
		err = r.handle(ctx, id, event)
	}
	r.registry.CompleteParent(r.owner, id, event, err == nil)
	r.observed <- sessionObservation{event: event, err: err}
	return plugins.Delivery{RequestID: id}, err
}
func takeSessionEvent(t *testing.T, r *sessionRuntime) chatevent.Event {
	t.Helper()
	select {
	case result := <-r.observed:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.event
	case <-time.After(2 * time.Second):
		t.Fatal("event was not delivered")
		return chatevent.Event{}
	}
}
func adapterMessage(id, text string) chatevent.NormalizedEvent {
	return chatevent.NormalizedEvent{Kind: chatevent.EventKindMessageText, EventID: id, BotID: "bot", SourceProtocol: "onebot11", SourceAdapter: "adapter", EventType: "message.group", Timestamp: time.Now().Unix(), ConversationType: "group", ConversationID: "group", SenderID: "actor", ActorRole: "member", PlainText: text}
}

func TestIngressConversationThreeRoundsAndExistingCommandPolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := conversation.New(conversation.Options{})
	t.Cleanup(r.Close)
	d := dispatch.New(logger, nil, nil, 16)
	t.Cleanup(d.Close)
	owner := conversation.Owner{PluginID: "p", Done: make(chan struct{})}
	turns := 0
	p := &sessionRuntime{owner: owner, registry: r, observed: make(chan sessionObservation, 8)}
	p.handle = func(ctx context.Context, parent string, event chatevent.Event) error {
		request := conversation.WaitRequest{}
		if event.Session != nil {
			turns++
			request.SessionID = event.Session.SessionID
			if turns == 3 {
				return nil
			}
		}
		_, err := r.Wait(ctx, owner, parent, event, request)
		return err
	}
	observer := &sessionRuntime{owner: conversation.Owner{PluginID: "observer", Done: make(chan struct{})}, registry: r, observed: make(chan sessionObservation, 8)}
	commands := []plugins.Command{{Name: "start", Permission: "everyone"}, {Name: "restricted", Permission: "super_admin"}}
	d.Register("p", p, []string{"message.group"}, commands, 1)
	d.Register("observer", observer, []string{"message.group"}, nil, 1)
	b := bridge.New(logger, d)
	cfg := config.Config{Command: &config.CommandConfig{Prefixes: []string{"/"}}, User: config.UserConfig{CommandRateLimit: "1/1h"}, Group: config.GroupConfig{CommandRateLimit: "100/1h"}}
	pluginsView := catalog.New([]plugins.Snapshot{{PluginID: "p", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running", Commands: commands}})
	ingress := chatpolicy.NewIngress(chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return cfg }, Logger: logger, Plugins: pluginsView, Bridge: b, Conversations: r})
	ingress.HandleAdapterEvent(t.Context(), adapterMessage("initial", "/start"))
	takeSessionEvent(t, p)
	var id string
	for i, text := range []string{"choice", "/restricted", "confirm"} {
		ingress.HandleAdapterEvent(t.Context(), adapterMessage(fmt.Sprint(i), text))
		event := takeSessionEvent(t, p)
		if event.Session == nil {
			t.Fatal("waiting reply used ordinary dispatch")
		}
		if id == "" {
			id = event.Session.SessionID
		} else if id != event.Session.SessionID {
			t.Fatal("conversation ID changed")
		}
		if event.PayloadFields["command"] != nil {
			t.Fatal("reply inherited command execution metadata")
		}
	}
	if r.HasWaiting(chatevent.FromAdapter(adapterMessage("check", ""))) {
		t.Fatal("completed session retained waiting")
	}
	ingress.HandleAdapterEvent(t.Context(), adapterMessage("denied", "/restricted"))
	ingress.HandleAdapterEvent(t.Context(), adapterMessage("cooldown", "/start"))
	if b.Snapshot().DeliveredCount != 4 {
		t.Fatalf("reply accounting or original policy changed: %#v", b.Snapshot())
	}
	select {
	case <-p.observed:
		t.Fatal("normal command bypassed original authorization/cooldown")
	default:
	}
	select {
	case <-observer.observed:
		t.Fatal("conversation reply fanned out")
	default:
	}
}

func TestIngressRegisteredAndHandlingInputsUseOrdinaryFlow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := conversation.New(conversation.Options{})
	t.Cleanup(r.Close)
	d := dispatch.New(logger, nil, nil, 16)
	t.Cleanup(d.Close)
	owner := conversation.Owner{PluginID: "p", Done: make(chan struct{})}
	started, release := make(chan struct{}), make(chan struct{})
	p := &sessionRuntime{owner: owner, registry: r, observed: make(chan sessionObservation, 8), handle: func(ctx context.Context, _ string, event chatevent.Event) error {
		if event.Session != nil {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}}
	observer := &sessionRuntime{owner: conversation.Owner{PluginID: "observer", Done: make(chan struct{})}, registry: r, observed: make(chan sessionObservation, 8)}
	d.Register("p", p, []string{"message.group"}, nil, 1)
	d.Register("observer", observer, []string{"message.group"}, nil, 1)
	ingress := chatpolicy.NewIngress(chatpolicy.IngressDeps{Logger: logger, Bridge: bridge.New(logger, d), Conversations: r})
	initial := chatevent.FromAdapter(adapterMessage("initial", "question"))
	if _, err := r.Wait(t.Context(), owner, "pending", initial, conversation.WaitRequest{}); err != nil {
		t.Fatal(err)
	}
	ingress.HandleAdapterEvent(t.Context(), adapterMessage("early", "early"))
	if takeSessionEvent(t, p).Session != nil || takeSessionEvent(t, observer).Session != nil {
		t.Fatal("registration buffered or captured early input")
	}
	r.CompleteParent(owner, "pending", initial, true)
	ingress.HandleAdapterEvent(t.Context(), adapterMessage("reply", "reply"))
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("reply did not start")
	}
	ingress.HandleAdapterEvent(t.Context(), adapterMessage("during", "during"))
	if event := takeSessionEvent(t, observer); event.EventID != "during" || event.Session != nil {
		t.Fatal("handling input was buffered")
	}
	close(release)
	if takeSessionEvent(t, p).Session == nil || takeSessionEvent(t, p).Session != nil {
		t.Fatal("reply/ordinary ownership changed")
	}
}

func TestBlacklistedReplyDoesNotCloseWaiting(t *testing.T) {
	r := conversation.New(conversation.Options{})
	t.Cleanup(r.Close)
	owner := conversation.Owner{PluginID: "p", Done: make(chan struct{})}
	event := adapterMessage("initial", "question")
	native := chatevent.FromAdapter(event)
	if _, err := r.Wait(t.Context(), owner, "parent", native, conversation.WaitRequest{}); err != nil {
		t.Fatal(err)
	}
	r.CompleteParent(owner, "parent", native, true)
	blacklist := newStubBlacklistRepo()
	blacklist.blockUser("actor")
	ingress := chatpolicy.NewIngress(chatpolicy.IngressDeps{Conversations: r, BlacklistRepo: blacklist})
	ingress.HandleAdapterEvent(t.Context(), adapterMessage("blocked", "reply"))
	if !r.HasWaiting(native) {
		t.Fatal("blacklisting actively closed a conversation")
	}
}
