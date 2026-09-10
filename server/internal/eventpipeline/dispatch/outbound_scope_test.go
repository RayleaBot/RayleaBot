package dispatch

import (
	"context"
	"log/slog"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
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
	allowAllPermissions(d)
	var recorded error
	d.SetOutboundPolicy(outboundPolicyFunc(func(context.Context, outbound.MessageLimitRequest) (outbound.MessageAdmission, error) {
		// Simulate a configuration change while quota admission was waiting.
		cfg.Adapters[0].Enabled, cfg.Adapters[1].Enabled = false, true
		return outbound.MessageAdmission{
			Scope:  chatevent.IdentityScope{Kind: "instance", SourceProtocol: "qqofficial", SourceAdapter: "a", BotID: "bot-a"},
			Record: func(err error) { recorded = err },
		}, nil
	}))
	_, err := d.ExecuteOutboundAction(t.Context(), "fixture", "request", chatevent.Event{}, chatevent.MessageCommand{
		Kind: "message.send", TargetType: "private", TargetID: "shared-id",
		MessageSegments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "fixture"}}},
	})
	if err == nil || recorded == nil {
		t.Fatalf("disabled admitted adapter was not rejected: send=%v record=%v", err, recorded)
	}
	if len(first.messages) != 0 || len(second.messages) != 0 {
		t.Fatal("pending send was rerouted to another adapter")
	}
}
