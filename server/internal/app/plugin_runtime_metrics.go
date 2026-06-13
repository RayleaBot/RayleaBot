package app

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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

func refreshPluginRuntimeStateGauge(registry *metrics.Registry, catalog *plugins.Catalog) {
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

func startPluginRuntimeStateGaugeRefresh(registry *metrics.Registry, catalog *plugins.Catalog) (stop func()) {
	if registry == nil || catalog == nil {
		return func() {}
	}
	refreshPluginRuntimeStateGauge(registry, catalog)
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
				refreshPluginRuntimeStateGauge(registry, catalog)
			case <-ticker.C:
				refreshPluginRuntimeStateGauge(registry, catalog)
			}
		}
	}()
	return func() {
		unsubscribe()
		<-done
	}
}
