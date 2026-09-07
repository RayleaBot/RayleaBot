package wsevents

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
)

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
	view := NewProtocolService(adapterConfigSource{}, ProtocolServiceAdapters{}).Adapters()
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
	service := NewProtocolService(adapterConfigSource{cfg: cfg}, ProtocolServiceAdapters{})
	qq := findAdapter(t, service.Adapters(), config.DefaultQQOfficialAdapterID)
	if qq.Enabled || qq.State != qqofficial.StateIdle || qq.Summary == "" {
		t.Fatalf("disabled adapter = %+v, want an idle disabled instance with a summary", qq)
	}
	if qq.Identity != nil {
		t.Fatal("an adapter that never connected reported a bot identity")
	}

	qqAdapter.Enabled = true
	cfg = config.Config{Adapters: []config.AdapterInstance{qqAdapter}}
	connected := NewProtocolService(adapterConfigSource{cfg: cfg}, ProtocolServiceAdapters{
		QQOfficial: map[string]QQOfficialAdapter{
			config.DefaultQQOfficialAdapterID: stubQQStatus{status: qqofficial.Status{
				State: qqofficial.StateConnected, Summary: "已连接：洛箐箐", BotID: "bot-1", BotName: "洛箐箐",
			}},
		},
	})
	qq = findAdapter(t, connected.Adapters(), config.DefaultQQOfficialAdapterID)
	if qq.State != qqofficial.StateConnected || qq.Identity == nil || qq.Identity.ID != "bot-1" {
		t.Fatalf("connected adapter = %+v, want the live state and identity", qq)
	}
}

func TestAdaptersKeepInstancesOfOneProtocolApart(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Adapters: []config.AdapterInstance{
		{ID: config.DefaultOneBot11AdapterID, Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &config.OneBotConfig{}},
		{ID: "second-bot", Type: config.AdapterTypeOneBot11, Enabled: false, OneBot11: &config.OneBotConfig{}},
	}}
	view := NewProtocolService(adapterConfigSource{cfg: cfg}, ProtocolServiceAdapters{}).Adapters()

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
