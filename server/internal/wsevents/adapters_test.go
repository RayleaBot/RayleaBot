package wsevents

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
)

type stubQQStatus struct{ status qqofficial.Status }

func (s stubQQStatus) Status() qqofficial.Status { return s.status }

type adapterConfigSource struct{ cfg config.Config }

func (s adapterConfigSource) CurrentConfig() config.Config { return s.cfg }

func TestAdaptersListsUnconfiguredAdaptersAsAddable(t *testing.T) {
	t.Parallel()

	// Nothing configured and no QQ client built: the adapter must still be
	// listed, otherwise the management surface cannot offer it as something to
	// add.
	service := NewProtocolService(adapterConfigSource{}, nil, nil)
	adapters := service.Adapters()
	if len(adapters) != 2 {
		t.Fatalf("listed %d adapters, want both formally supported ones", len(adapters))
	}

	byProtocol := map[string]AdapterDescriptor{}
	for _, adapter := range adapters {
		byProtocol[adapter.Protocol] = adapter
	}
	qq, ok := byProtocol["qqofficial"]
	if !ok {
		t.Fatal("qqofficial was omitted from the listing")
	}
	if qq.Configured || qq.Enabled {
		t.Fatalf("unconfigured adapter reported configured=%v enabled=%v", qq.Configured, qq.Enabled)
	}
	if qq.State != qqofficial.StateIdle || qq.Summary == "" {
		t.Fatalf("unconfigured adapter state/summary = %q/%q", qq.State, qq.Summary)
	}
	if qq.Identity != nil {
		t.Fatal("an adapter that never connected reported a bot identity")
	}
}

func TestAdaptersDistinguishConfiguredFromEnabledAndConnected(t *testing.T) {
	t.Parallel()

	cfg := config.Config{QQOfficial: config.QQOfficialConfig{
		AppID: "102209770", AppSecret: "secret://qq_official/app_secret", Enabled: false,
	}}
	// Configured but switched off: the operator needs to see that this is a
	// choice, not a failure.
	service := NewProtocolService(adapterConfigSource{cfg: cfg}, nil, nil)
	qq := findAdapter(t, service.Adapters(), "qqofficial")
	if !qq.Configured || qq.Enabled {
		t.Fatalf("configured-but-disabled reported configured=%v enabled=%v", qq.Configured, qq.Enabled)
	}

	cfg.QQOfficial.Enabled = true
	connected := NewProtocolService(adapterConfigSource{cfg: cfg}, nil, stubQQStatus{status: qqofficial.Status{
		State: qqofficial.StateConnected, Summary: "已连接：洛箐箐", BotID: "bot-1", BotName: "洛箐箐",
	}})
	qq = findAdapter(t, connected.Adapters(), "qqofficial")
	if qq.State != qqofficial.StateConnected || qq.Identity == nil || qq.Identity.ID != "bot-1" {
		t.Fatalf("connected adapter = %+v, want the live state and identity", qq)
	}
}

func findAdapter(t *testing.T, adapters []AdapterDescriptor, protocol string) AdapterDescriptor {
	t.Helper()
	for _, adapter := range adapters {
		if adapter.Protocol == protocol {
			return adapter
		}
	}
	t.Fatalf("adapter %q not listed", protocol)
	return AdapterDescriptor{}
}
