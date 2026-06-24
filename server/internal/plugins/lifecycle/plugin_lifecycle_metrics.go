package lifecycle

import (
	"time"

	coremetrics "github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

var pluginStates = []string{
	plugins.PluginStateDisabled,
	plugins.PluginStateEnabled,
	plugins.PluginStateStarting,
	plugins.PluginStateRunning,
	plugins.PluginStateStopping,
	plugins.PluginStateFailed,
	plugins.PluginStateInvalid,
}

func RefreshPluginStateGauge(registry *coremetrics.Registry, catalog *plugincatalog.Catalog) {
	if registry == nil || registry.PluginState == nil || catalog == nil {
		return
	}
	counts := make(map[string]int, len(pluginStates))
	for _, state := range pluginStates {
		counts[state] = 0
	}
	for _, snapshot := range catalog.List() {
		state, _ := plugins.ProjectState(snapshot)
		if _, ok := counts[state]; !ok {
			counts[state] = 0
		}
		counts[state]++
	}
	for state, count := range counts {
		registry.PluginState.WithLabelValues(state).Set(float64(count))
	}
}

func StartPluginStateGaugeRefresh(registry *coremetrics.Registry, catalog *plugincatalog.Catalog) (stop func()) {
	if registry == nil || catalog == nil {
		return func() {}
	}
	RefreshPluginStateGauge(registry, catalog)
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
				RefreshPluginStateGauge(registry, catalog)
			case <-ticker.C:
				RefreshPluginStateGauge(registry, catalog)
			}
		}
	}()
	return func() {
		unsubscribe()
		<-done
	}
}
