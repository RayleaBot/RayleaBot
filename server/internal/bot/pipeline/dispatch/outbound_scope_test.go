package dispatch

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

type outboundPolicyFunc func(context.Context, outbound.MessageLimitRequest) (outbound.MessageAdmission, error)

func (f outboundPolicyFunc) Begin(ctx context.Context, request outbound.MessageLimitRequest) (outbound.MessageAdmission, error) {
	return f(ctx, request)
}

func TestOutboundAdmissionDoesNotRerouteAfterAdapterSwitch(t *testing.T) {
	first, second := &fakeSender{}, &fakeSender{}
	cfg := config.Config{Adapters: []config.AdapterInstance{
		{ID: "a", Type: "qqofficial", Enabled: true},
		{ID: "b", Type: "qqofficial", Enabled: false},
	}}
	router := outbound.NewRouter(map[string]outbound.ActionSender{"a": first, "b": second}, map[string]string{"a": "qqofficial", "b": "qqofficial"}, func() config.Config { return cfg })
	d := New(slog.Default(), router, nil, 1)
	t.Cleanup(d.Close)
	d.SetOutboundPolicy(outboundPolicyFunc(func(context.Context, outbound.MessageLimitRequest) (outbound.MessageAdmission, error) {
		// Simulate a configuration change while quota admission was waiting.
		cfg.Adapters[0].Enabled, cfg.Adapters[1].Enabled = false, true
		return outbound.MessageAdmission{
			Scope: chatevent.IdentityScope{Kind: "instance", SourceProtocol: "qqofficial", SourceAdapter: "a", BotID: "bot-a"},
		}, nil
	}))
	_, err := d.ExecuteOutboundAction(t.Context(), "fixture", "request", chatevent.Event{}, chatevent.MessageCommand{
		Kind: "message.send", TargetType: "private", TargetID: "shared-id",
		MessageSegments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "fixture"}}},
	})
	if err == nil {
		t.Fatalf("disabled admitted adapter was not rejected: send=%v", err)
	}
	if len(first.messages) != 0 || len(second.messages) != 0 {
		t.Fatal("pending send was rerouted to another adapter")
	}
}

func TestOutboundAdmissionPreservesChatParentAdapter(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"enabled", "disabled", "removed"} {
		t.Run(state, func(t *testing.T) {
			first, second := &fakeSender{}, &fakeSender{}
			cfg := config.Config{Adapters: []config.AdapterInstance{
				{ID: "onebot11", Type: "onebot11", Enabled: true},
				{ID: "qq-official", Type: "qqofficial", Enabled: state == "enabled"},
			}}
			if state == "removed" {
				cfg.Adapters = cfg.Adapters[:1]
			}
			router := outbound.NewRouter(map[string]outbound.ActionSender{"onebot11": first, "qq-official": second},
				map[string]string{"onebot11": "onebot11", "qq-official": "qqofficial"}, func() config.Config { return cfg })
			d := New(slog.Default(), router, nil, 1)
			t.Cleanup(d.Close)
			d.SetOutboundPolicy(outbound.NewMessagePolicy(cfg, func(scope chatevent.IdentityScope) chatevent.IdentityScope {
				if scope.SourceAdapter != "qq-official" || scope.SourceProtocol != "qqofficial" {
					t.Errorf("quota admission lost the parent adapter: %+v", scope)
				}
				return router.ResolveScope(scope, nil)
			}))
			_, err := d.ExecuteOutboundAction(t.Context(), "fixture", "request",
				chatevent.Event{SourceAdapter: "qq-official", SourceProtocol: "qqofficial", EventType: "message.private"},
				chatevent.MessageCommand{Kind: "message.send", TargetType: "private", TargetID: "U1"})
			if len(first.messages) != 0 {
				t.Fatal("send was rerouted to another adapter")
			}
			if state == "enabled" {
				if err != nil || len(second.messages) != 1 {
					t.Fatalf("parent adapter did not send: error=%v, sent=%v", err, second.messages)
				}
				return
			}
			var sendErr *chatevent.SendError
			if !errors.As(err, &sendErr) || sendErr.Code != errorcodes.AdapterTransportUnavailable || len(second.messages) != 0 {
				t.Fatalf("unavailable parent: error=%v, sent=%v", err, second.messages)
			}
		})
	}
}
