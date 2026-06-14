package service

import (
	"strings"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
)

// pluginRuntimeStates enumerates formal runtime states so stale gauge buckets reset to zero.
var pluginRuntimeStates = []string{
	"stopped",
	"starting",
	"running",
	"stopping",
	"crashed",
	"backoff",
	"dead_letter",
}

func RefreshPluginRuntimeStateGauge(registry *metrics.Registry, catalog *plugincatalog.Catalog) {
	if registry == nil || registry.PluginRuntimeState == nil || catalog == nil {
		return
	}
	counts := make(map[string]int, len(pluginRuntimeStates))
	for _, state := range pluginRuntimeStates {
		counts[state] = 0
	}
	for _, snapshot := range catalog.List() {
		state := strings.TrimSpace(snapshot.RuntimeState)
		if state == "" {
			continue
		}
		if _, ok := counts[state]; !ok {
			counts[state] = 0
		}
		counts[state]++
	}
	for state, count := range counts {
		registry.PluginRuntimeState.WithLabelValues(state).Set(float64(count))
	}
}

func StartPluginRuntimeStateGaugeRefresh(registry *metrics.Registry, catalog *plugincatalog.Catalog) (stop func()) {
	if registry == nil || catalog == nil {
		return func() {}
	}
	RefreshPluginRuntimeStateGauge(registry, catalog)
	events, unsubscribe := catalog.Subscribe(16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case _, ok := <-events:
				if !ok {
					return
				}
				RefreshPluginRuntimeStateGauge(registry, catalog)
			case <-ticker.C:
				RefreshPluginRuntimeStateGauge(registry, catalog)
			}
		}
	}()
	return func() {
		unsubscribe()
		<-done
	}
}
