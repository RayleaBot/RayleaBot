package wsevents

import (
	"context"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
)

func TestAdapterSnapshotsShareCollectionOrderAndInstanceDetails(t *testing.T) {
	t.Parallel()
	settings := config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: "https://second.example/api"}}
	source := &adapterConfigSource{cfg: config.Config{Adapters: []config.AdapterInstance{
		{ID: "first", Type: "onebot11", Enabled: false, OneBot11: &config.OneBotConfig{}},
		{ID: "second", Type: "onebot11", Enabled: true, OneBot11: &settings},
		{ID: "qq", Type: "qqofficial", Enabled: true, QQOfficial: &config.QQOfficialConfig{}},
	}}}
	service := newTestService(t, source, ProtocolServiceAdapters{
		OneBot11: map[string]*onebot11.Shell{
			"first":  onebot11.New("first", config.OneBotConfig{}, config.AdapterConfig{}, nil),
			"second": onebot11.New("second", settings, config.AdapterConfig{}, nil),
		},
		QQOfficial: map[string]QQOfficialAdapter{"qq": stubQQStatus{status: qqofficial.Status{State: "connected", BotID: "opaque-qq"}}},
	})
	for _, stage := range []string{"configured", "reordered", "QQ only", "empty"} {
		switch stage {
		case "reordered":
			source.cfg.Adapters[0], source.cfg.Adapters[1] = source.cfg.Adapters[1], source.cfg.Adapters[0]
		case "QQ only":
			source.cfg.Adapters = source.cfg.Adapters[2:]
		case "empty":
			source.cfg.Adapters = nil
		}
		view := service.Adapters().Adapters
		frame := service.Adapters()
		updates, unsubscribe := service.SubscribeProtocolEvents(1)
		service.PublishSnapshot()
		published := (<-updates).Data.(AdaptersSnapshotPayload)
		unsubscribe()
		if !reflect.DeepEqual(frame.Adapters, published.Adapters) {
			t.Fatal("initial and live adapter snapshots differ")
		}
		states := service.AdapterStates()
		if !reflect.DeepEqual(view, frame.Adapters) || len(states) != len(view) || view == nil || states == nil {
			t.Fatalf("%s: HTTP, WS and system collections differ: %#v %#v %#v", stage, view, frame, states)
		}
		for index, instance := range source.cfg.Adapters {
			descriptor, state := view[index], states[index]
			if descriptor.ID != instance.ID || state.ID != descriptor.ID || state.Enabled != descriptor.Enabled || state.State != descriptor.State || state.Protocol != descriptor.Protocol {
				t.Fatalf("%s: instance or state changed across projections: %#v %#v", stage, descriptor, state)
			}
			if instance.ID == "second" && (descriptor.OneBot11 == nil || descriptor.OneBot11.TransportStatus[2].Endpoint != "https://second.example") {
				t.Fatalf("second instance lost its own transport: %#v", descriptor.OneBot11)
			}
			if instance.Type == "qqofficial" && (descriptor.OneBot11 != nil || descriptor.State != "connected") {
				t.Fatalf("QQ-only status contains a placeholder OneBot: %#v", descriptor)
			}
		}
	}
}

type stubQQStatus struct{ status qqofficial.Status }

func (s stubQQStatus) Status() qqofficial.Status { return s.status }

func (stubQQStatus) Reload(config.QQOfficialConfig) bool { return false }
func (stubQQStatus) SetEnabled(bool)                     {}

type adapterConfigSource struct{ cfg config.Config }

func (s adapterConfigSource) CurrentConfig() config.Config { return s.cfg }

func TestAdaptersOffersEveryProtocolWhenNothingIsConfigured(t *testing.T) {
	t.Parallel()

	// A fresh install has no adapter instances, so the listing is empty and the
	// page has nothing to show unless the protocols are offered separately.
	view := newTestService(t, adapterConfigSource{}, ProtocolServiceAdapters{}).Adapters()
	if len(view.Adapters) != 0 {
		t.Fatalf("listed %d adapters, want none before any is added", len(view.Adapters))
	}
	protocols := map[string]bool{}
	for _, protocol := range view.AvailableProtocols {
		protocols[protocol.Protocol] = true
		if protocol.DisplayName == "" || protocol.Description == "" {
			t.Fatalf("protocol %q offered without a name or description", protocol.Protocol)
		}
	}
	for _, want := range []string{config.AdapterTypeOneBot11, config.AdapterTypeQQOfficial} {
		if !protocols[want] {
			t.Fatalf("protocol %q was not offered as addable", want)
		}
	}
}

func TestAdaptersReportEnabledStateAndLiveIdentityPerInstance(t *testing.T) {
	t.Parallel()

	qqAdapter := config.AdapterInstance{
		ID:      config.DefaultQQOfficialAdapterID,
		Type:    config.AdapterTypeQQOfficial,
		Enabled: false,
		QQOfficial: &config.QQOfficialConfig{
			AppID:     "100000001",
			AppSecret: "secret://adapters/qq-official/qqofficial/app_secret",
		},
	}
	cfg := config.Config{Adapters: []config.AdapterInstance{qqAdapter}}

	// Added but switched off: the operator needs to see that this is a choice,
	// not a failure, and no client is running to report a state.
	service := newTestService(t, adapterConfigSource{cfg: cfg}, ProtocolServiceAdapters{})
	qq := findAdapter(t, service.Adapters(), config.DefaultQQOfficialAdapterID)
	if qq.Enabled || qq.State != qqofficial.StateIdle || qq.Summary == "" {
		t.Fatalf("disabled adapter = %+v, want an idle disabled instance with a summary", qq)
	}
	if qq.Identity != nil {
		t.Fatal("an adapter that never connected reported a bot identity")
	}

	qqAdapter.Enabled = true
	cfg = config.Config{Adapters: []config.AdapterInstance{qqAdapter}}
	connected := newTestService(t, adapterConfigSource{cfg: cfg}, ProtocolServiceAdapters{
		QQOfficial: map[string]QQOfficialAdapter{
			config.DefaultQQOfficialAdapterID: stubQQStatus{status: qqofficial.Status{
				State: qqofficial.StateConnected, Summary: "已连接：洛箐箐", BotID: "bot-1", BotName: "洛箐箐", BotAvatarURL: "https://example.com/bot.png",
			}},
		},
	})
	qq = findAdapter(t, connected.Adapters(), config.DefaultQQOfficialAdapterID)
	if qq.State != qqofficial.StateConnected || qq.Identity == nil || qq.Identity.ID != "bot-1" || qq.Identity.Name != "洛箐箐" || qq.Identity.AvatarURL != "https://example.com/bot.png" {
		t.Fatalf("connected adapter = %+v, want the live state and identity", qq)
	}
}

func TestAdaptersKeepInstancesOfOneProtocolApart(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Adapters: []config.AdapterInstance{
		{ID: config.DefaultOneBot11AdapterID, Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &config.OneBotConfig{}},
		{ID: "second-bot", Type: config.AdapterTypeOneBot11, Enabled: false, OneBot11: &config.OneBotConfig{}},
	}}
	view := newTestService(t, adapterConfigSource{cfg: cfg}, ProtocolServiceAdapters{}).Adapters()

	if len(view.Adapters) != 2 {
		t.Fatalf("listed %d adapters, want both instances", len(view.Adapters))
	}
	// Two instances of one protocol have to be distinguishable on the page, so
	// the display name of an added instance carries its identifier.
	if view.Adapters[0].DisplayName == view.Adapters[1].DisplayName {
		t.Fatalf("both instances rendered as %q", view.Adapters[0].DisplayName)
	}
	if view.Adapters[1].ID != "second-bot" || view.Adapters[1].Enabled {
		t.Fatalf("second instance = %+v, want the disabled added instance", view.Adapters[1])
	}
}

func findAdapter(t *testing.T, view AdaptersView, id string) AdapterDescriptor {
	t.Helper()
	for _, adapter := range view.Adapters {
		if adapter.ID == id {
			return adapter
		}
	}
	t.Fatalf("adapter %q not listed", id)
	return AdapterDescriptor{}
}

func TestOneBotIdentityUsesConfirmedAccountAndMatchingLoginInfo(t *testing.T) {
	t.Parallel()
	connected := func(id, name string) onebot11.TransportSnapshot {
		return onebot11.TransportSnapshot{Enabled: true, State: onebot11.TransportStateConnected, RuntimeInfo: onebot11.TransportRuntimeInfo{UserID: id, Nickname: name}}
	}
	for _, tt := range []struct {
		name     string
		snapshot onebot11.Snapshot
		want     *AdapterIdentity
	}{
		{"unknown", onebot11.Snapshot{}, nil},
		{"webhook only", onebot11.Snapshot{BotID: "10001"}, &AdapterIdentity{ID: "10001", AvatarURL: oneBot11AvatarURL("10001")}},
		{"HTTP login before event", onebot11.Snapshot{HTTPAPI: connected("10002", "second")}, &AdapterIdentity{ID: "10002", Name: "second", AvatarURL: oneBot11AvatarURL("10002")}},
		{"matching login", onebot11.Snapshot{BotID: "10001", ReverseWS: connected("10001", "first")}, &AdapterIdentity{ID: "10001", Name: "first", AvatarURL: oneBot11AvatarURL("10001")}},
		{"other transport account", onebot11.Snapshot{BotID: "10001", ReverseWS: connected("10002", "wrong"), HTTPAPI: connected("10001", "right")}, &AdapterIdentity{ID: "10001", Name: "right", AvatarURL: oneBot11AvatarURL("10001")}},
		{"disconnected login", onebot11.Snapshot{ForwardWS: onebot11.TransportSnapshot{Enabled: true, State: onebot11.TransportStateStopped, RuntimeInfo: onebot11.TransportRuntimeInfo{UserID: "10001", Nickname: "stale"}}}, nil},
		{"non QQ identifier", onebot11.Snapshot{BotID: "opaque-bot"}, &AdapterIdentity{ID: "opaque-bot"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := oneBot11Identity(tt.snapshot)
			if got == nil || tt.want == nil {
				if got != tt.want {
					t.Fatalf("identity = %+v, want %+v", got, tt.want)
				}
			} else if *got != *tt.want {
				t.Fatalf("identity = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func (stubQQStatus) Start(context.Context)      {}
func (stubQQStatus) Stop(context.Context) error { return nil }
