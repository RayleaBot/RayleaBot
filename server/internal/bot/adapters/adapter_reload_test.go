package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestInstanceSwitchRestoresConfiguredIngressWithoutRestart(t *testing.T) {
	settings := config.OneBotConfig{ReverseWS: config.OneBotTransportConfig{Enabled: true, URL: "ws://127.0.0.1/fixture"}}
	cfg := config.Config{Adapters: []config.AdapterInstance{{ID: "fixture", Type: "onebot11", OneBot11: &settings}}}
	effective, _ := cfg.OneBot11RuntimeSettings("fixture")
	shell := onebot11.New("fixture", effective, cfg.Adapter, nil)
	source := &adapterConfigSource{cfg: cfg}
	service := newTestService(t, source, Instances{OneBot11: map[string]*onebot11.Shell{"fixture": shell}})
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

func TestNewQQInstanceStartsAndStopsOnReload(t *testing.T) {
	cfg := config.Config{Adapters: []config.AdapterInstance{{ID: "new-qq", Type: "qqofficial", Enabled: true, QQOfficial: &config.QQOfficialConfig{AppID: "1001"}}}}
	client := &lifecycleQQ{stop: func(context.Context) error { return nil }}
	service := newTestService(t, adapterConfigSource{}, Instances{
		NewQQOfficial: func(id string, settings config.QQOfficialConfig, _ config.AdapterConfig) QQOfficialAdapter {
			if id != "new-qq" || settings.AppID != "1001" {
				t.Fatalf("factory received %s, %+v", id, settings)
			}
			return client
		},
	})
	if err := service.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	for range 2 {
		if err := service.ApplyConfigReload(cfg); err != nil {
			t.Fatal(err)
		}
	}
	if client.starts.Load() != 1 || client.stops.Load() != 0 {
		t.Fatal("new QQ instance was not started exactly once")
	}
	if err := service.ApplyConfigReload(config.Config{}); err != nil {
		t.Fatal(err)
	}
	if client.stops.Load() != 1 || service.registry.Snapshot().QQOfficial("new-qq") != nil {
		t.Fatal("removed QQ runtime survived")
	}
}

func TestQQAppIDChangeReplacesRuntimeGeneration(t *testing.T) {
	old := &lifecycleQQ{stop: func(context.Context) error { return nil }}
	replacement := &lifecycleQQ{stop: func(context.Context) error { return nil }}
	settings := config.QQOfficialConfig{AppID: "old-app"}
	cfg := config.Config{Adapters: []config.AdapterInstance{{ID: "qq", Type: "qqofficial", Enabled: true, QQOfficial: &settings}}}
	service := newTestService(t, adapterConfigSource{cfg: cfg}, Instances{
		QQOfficial:    map[string]QQOfficialAdapter{"qq": old},
		NewQQOfficial: func(string, config.QQOfficialConfig, config.AdapterConfig) QQOfficialAdapter { return replacement },
	})
	if err := service.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	next := config.QQOfficialConfig{AppID: "new-app"}
	cfg.Adapters[0].QQOfficial = &next
	if err := service.ApplyConfigReload(cfg); err != nil {
		t.Fatal(err)
	}
	if service.registry.Snapshot().QQOfficial("qq") != replacement || old.stops.Load() != 1 || replacement.starts.Load() != 1 {
		t.Fatal("AppID change reused the retired runtime")
	}
}
