package adapters

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

func TestAdapterSnapshotsKeepLatestAndIsolateSubscribers(t *testing.T) {
	settings := config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: "https://fixture.example/api"}}
	source := &adapterConfigSource{cfg: config.Config{Adapters: []config.AdapterInstance{{ID: "fixture", Type: "onebot11", OneBot11: &settings}}}}
	shell := onebot11.New("fixture", settings, config.AdapterConfig{}, nil)
	instances := Instances{OneBot11: map[string]*onebot11.Shell{"fixture": shell}}
	service := newTestService(t, source, instances)
	delete(instances.OneBot11, "fixture")
	service.PublishSnapshot()
	initial, first, unsubscribeFirst := service.SnapshotAndSubscribe(1)
	defer unsubscribeFirst()
	_, second, unsubscribeSecond := service.SnapshotAndSubscribe(1)
	defer unsubscribeSecond()
	if initial.Adapters[0].OneBot11 == nil {
		t.Fatal("caller mutation removed an owned adapter instance")
	}
	select {
	case <-first:
		t.Fatal("initial snapshot was followed by a previously published state")
	default:
	}
	service.PublishSnapshot()
	source.cfg.Adapters[0].Enabled = true
	service.PublishSnapshot()
	a, b := <-first, <-second
	if !a.Adapters[0].Enabled || !b.Adapters[0].Enabled {
		t.Fatal("slow subscribers retained obsolete state")
	}
	a.Adapters[0].OneBot11.TransportStatus[0].State = "corrupted"
	a.AvailableProtocols[0].Protocol = "corrupted"
	if b.Adapters[0].OneBot11.TransportStatus[0].State == "corrupted" || b.AvailableProtocols[0].Protocol == "corrupted" || service.Adapters().AvailableProtocols[0].Protocol == "corrupted" {
		t.Fatal("subscriber mutation escaped its snapshot")
	}
}

type lifecycleQQ struct {
	stubQQStatus
	starts atomic.Int32
	stops  atomic.Int32
	stop   func(context.Context) error
}

func (client *lifecycleQQ) Start(context.Context) { client.starts.Add(1) }
func (client *lifecycleQQ) Stop(ctx context.Context) error {
	client.stops.Add(1)
	return client.stop(ctx)
}

func TestStopOwnsEveryAdapterAndEndsSubscriptions(t *testing.T) {
	stopFailure := errors.New("fixture stop failed")
	first := &lifecycleQQ{stop: func(context.Context) error { return stopFailure }}
	second := &lifecycleQQ{stop: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}
	service := newTestService(t, adapterConfigSource{}, Instances{QQOfficial: map[string]QQOfficialAdapter{"first": first, "second": second}})
	_, updates, unsubscribe := service.SnapshotAndSubscribe(1)
	defer unsubscribe()
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Stop(ctx); !errors.Is(err, stopFailure) || !errors.Is(err, context.Canceled) {
		t.Fatalf("stop did not retain all adapter failures: %v", err)
	}
	if first.starts.Load() != 1 || second.starts.Load() != 1 || first.stops.Load() != 1 || second.stops.Load() != 1 {
		t.Fatal("one adapter failure prevented another adapter's lifecycle")
	}
	for {
		select {
		case _, open := <-updates:
			if !open {
				goto closed
			}
		case <-time.After(time.Second):
			t.Fatal("adapter subscription survived shutdown")
		}
	}
closed:
	if err := service.ApplyConfigReload(config.Config{}); !errors.Is(err, ErrStopped) {
		t.Fatalf("reload after stop: %v", err)
	}
	if err := service.Start(context.Background()); !errors.Is(err, ErrStopped) {
		t.Fatalf("start after stop: %v", err)
	}
}
