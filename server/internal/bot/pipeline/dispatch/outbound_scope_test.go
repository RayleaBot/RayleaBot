package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestOutboundAccountAdmissionBindsDeliveryToTheSelectedInstance(t *testing.T) {
	for _, change := range []string{"none", "reorder", "account changed", "disconnected", "unavailable"} {
		t.Run(change, func(t *testing.T) {
			first, second := &fakeSender{}, &fakeSender{}
			cfg := config.Config{Adapters: []config.AdapterInstance{
				{ID: "z", Type: "onebot11", Enabled: true}, {ID: "a", Type: "onebot11", Enabled: true},
			}}
			connected := map[string]string{"z": "10001", "a": "10001"}
			if change == "unavailable" {
				connected = nil
			}
			router := outbound.NewLiveRouter(func() outbound.Routes {
				return outbound.Routes{Config: &cfg, Senders: map[string]outbound.ActionSender{"z": first, "a": second}, Protocols: map[string]string{"z": "onebot11", "a": "onebot11"}, ConnectedBotIDs: connected}
			})
			policy := outbound.NewMessagePolicy(cfg, func(scope chatevent.IdentityScope) (chatevent.IdentityScope, error) {
				return router.ResolveScope(scope, nil)
			})
			var logs bytes.Buffer
			d := New(slog.New(slog.NewJSONHandler(&logs, nil)), router, nil, 1)
			t.Cleanup(d.Close)
			d.SetOutboundPolicy(outboundPolicyFunc(func(ctx context.Context, request outbound.MessageLimitRequest) (outbound.MessageAdmission, error) {
				admission, err := policy.Begin(ctx, request)
				if change == "unavailable" {
					var sendErr *chatevent.SendError
					if !errors.As(err, &sendErr) || sendErr.Code != errorcodes.AdapterTransportUnavailable {
						t.Fatalf("admitted an unavailable account: %+v, %v", admission, err)
					}
					return admission, err
				}
				if err != nil || admission.Scope.SourceAdapter != "z" || admission.Scope.BotID != "10001" {
					t.Fatalf("quota scope=%+v err=%v", admission.Scope, err)
				}
				switch change {
				case "reorder":
					cfg.Adapters[0], cfg.Adapters[1] = cfg.Adapters[1], cfg.Adapters[0]
				case "account changed":
					connected["z"] = "10002"
				case "disconnected":
					delete(connected, "z")
				}
				return admission, nil
			}))
			result, err := d.ExecuteOutboundAction(t.Context(), "fixture", "request", chatevent.Event{SourceProtocol: "onebot11", SourceAdapter: "a", BotID: "parent"},
				chatevent.MessageCommand{Kind: "message.send", SourceProtocol: "onebot11", BotID: "10001", TargetType: "group", TargetID: "200"})
			var entry struct {
				SourceAdapter string `json:"source_adapter"`
				BotID         string `json:"self_id"`
			}
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			wantAdapter := "z"
			if change == "unavailable" {
				wantAdapter = ""
			}
			if entry.SourceAdapter != wantAdapter || entry.BotID != "10001" {
				t.Fatalf("observation used another route: %+v", entry)
			}
			if len(second.messages) != 0 {
				t.Fatal("delivery switched away from the admitted instance")
			}
			if change == "none" || change == "reorder" {
				if err != nil || result.SourceAdapter != "z" || len(first.messages) != 1 {
					t.Fatalf("delivery=%+v err=%v messages=%v", result, err, first.messages)
				}
			} else {
				var sendErr *chatevent.SendError
				if !errors.As(err, &sendErr) || sendErr.Code != errorcodes.AdapterTransportUnavailable || len(first.messages) != 0 {
					t.Fatalf("unavailable route: err=%v messages=%v", err, first.messages)
				}
			}
		})
	}
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
			d.SetOutboundPolicy(outbound.NewMessagePolicy(cfg, func(scope chatevent.IdentityScope) (chatevent.IdentityScope, error) {
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
