package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// The router is keyed by instance id while events name a protocol, so this
// pins that the two still meet: routing on the protocol alone must reach the
// QQ adapter even though its instance is called something else.
func TestEventWiringRoutesOutboundToEachConfiguredAdapter(t *testing.T) {
	t.Parallel()

	state := buildEvents(eventDeps{
		Logger: discardLogger(),
		Config: config.Config{Adapters: []config.AdapterInstance{
			{ID: config.DefaultOneBot11AdapterID, Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &config.OneBotConfig{}},
			{ID: config.DefaultQQOfficialAdapterID, Type: config.AdapterTypeQQOfficial, Enabled: true,
				QQOfficial: &config.QQOfficialConfig{AppID: "100000001", AppSecret: "fixture-secret"}},
		}},
	})

	// Neither adapter is connected, so delivery fails either way. What this
	// asserts is that the router found one at all: a routing miss is a
	// different error from the adapter refusing the send.
	for _, testCase := range []struct {
		name    string
		message chatevent.OutboundMessageReply
	}{
		{
			name: "by instance id",
			message: chatevent.OutboundMessageReply{
				SourceAdapter: config.DefaultQQOfficialAdapterID, TargetType: "group", TargetID: "G1",
			},
		},
		{
			name: "by protocol when that protocol has one instance",
			message: chatevent.OutboundMessageReply{
				SourceProtocol: config.AdapterTypeQQOfficial, TargetType: "group", TargetID: "G1",
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := state.OutboundSender.SendReply(context.Background(), testCase.message)
			if err != nil && strings.Contains(err.Error(), "outbound:") {
				t.Fatalf("outbound routing failed to find the QQ adapter: %v", err)
			}
		})
	}
}

func TestEventWiringKeepsTwoInstancesOfOneProtocolApart(t *testing.T) {
	t.Parallel()

	state := buildEvents(eventDeps{
		Logger: discardLogger(),
		Config: config.Config{Adapters: []config.AdapterInstance{
			{ID: config.DefaultOneBot11AdapterID, Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &config.OneBotConfig{}},
			{ID: "second-bot", Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &config.OneBotConfig{}},
		}},
	})

	// Two instances speak the same protocol, so the protocol alone no longer
	// identifies a conversation and the router says so rather than guessing.
	_, err := state.OutboundSender.SendReply(context.Background(), chatevent.OutboundMessageReply{
		SourceProtocol: config.AdapterTypeOneBot11, TargetType: "group", TargetID: "G1",
	})
	if err == nil || !strings.Contains(err.Error(), "second-bot") {
		t.Fatalf("ambiguous protocol error = %v, want it to name both instances", err)
	}

	// Naming the instance resolves it.
	if _, err := state.OutboundSender.SendReply(context.Background(), chatevent.OutboundMessageReply{
		SourceAdapter: "second-bot", TargetType: "group", TargetID: "G1",
	}); err != nil && strings.Contains(err.Error(), "outbound:") {
		t.Fatalf("routing to the named instance failed: %v", err)
	}
}

// A disabled instance is a choice, not a failure: nothing is built for it, so
// it neither connects nor accepts a message addressed to it.
func TestEventWiringSkipsDisabledInstances(t *testing.T) {
	t.Parallel()

	state := buildEvents(eventDeps{
		Logger: discardLogger(),
		Config: config.Config{Adapters: []config.AdapterInstance{
			{ID: "off-bot", Type: config.AdapterTypeOneBot11, Enabled: false, OneBot11: &config.OneBotConfig{}},
		}},
	})
	if _, present := state.OneBotShells["off-bot"]; present {
		t.Fatal("a disabled instance was built")
	}
	_, err := state.OutboundSender.SendReply(context.Background(), chatevent.OutboundMessageReply{
		SourceAdapter: "off-bot", TargetType: "group", TargetID: "G1",
	})
	if err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("send to a disabled instance = %v, want it refused as not connected", err)
	}
}
