package wsevents

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

func TestInstanceSwitchRestoresConfiguredIngressWithoutRestart(t *testing.T) {
	settings := config.OneBotConfig{ReverseWS: config.OneBotTransportConfig{Enabled: true, URL: "ws://127.0.0.1/fixture"}}
	cfg := config.Config{Adapters: []config.AdapterInstance{{ID: "fixture", Type: "onebot11", OneBot11: &settings}}}
	effective, _ := cfg.OneBot11RuntimeSettings("fixture")
	shell := onebot11.New("fixture", effective, cfg.Adapter, nil)
	source := &adapterConfigSource{cfg: cfg}
	service := NewProtocolService(source, ProtocolServiceAdapters{OneBot11: map[string]*onebot11.Shell{"fixture": shell}})
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := shell.Stop(stopCtx); err != nil {
			t.Error(err)
		}
	}()
	shell.Start(ctx)
	for _, enabled := range []bool{true, false, true} {
		source.cfg.Adapters[0].Enabled = enabled
		if err := service.ApplyConfigReload(source.cfg); err != nil {
			t.Fatal(err)
		}
		ingress, ok := service.OneBot11Ingress("fixture")
		if ok != enabled {
			t.Fatalf("enabled=%v: ingress exists=%v", enabled, ok)
		}
		if enabled && !ingress.ReverseWSEnabled() {
			t.Fatal("configured reverse websocket was not restored")
		}
		if !settings.ReverseWS.Enabled {
			t.Fatal("runtime switch overwrote configured transport")
		}
	}
}

func TestNewInstanceSettingsStillRequireRestart(t *testing.T) {
	cfg := config.Config{Adapters: []config.AdapterInstance{{ID: "new-qq", Type: "qqofficial", Enabled: true, QQOfficial: &config.QQOfficialConfig{AppID: "1001"}}}}
	service := NewProtocolService(adapterConfigSource{cfg: cfg}, ProtocolServiceAdapters{})
	if err := service.ApplyConfigReload(cfg); err == nil {
		t.Fatal("unbuilt instance was reported as reloaded")
	}
}
