package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
)

func wireAppMetrics(platform appPlatform, pluginStack appPlugins) (*metrics.Registry, func()) {
	registry := metrics.New()
	pluginStack.Bridge.SetMetricsObserver(bridgeMetricsAdapter{registry: registry})
	pluginStack.Dispatcher.SetMetricsObserver(dispatchMetricsAdapter{registry: registry})
	pluginStack.Adapter.SetMetricsObserver(adapterMetricsAdapter{registry: registry})
	platform.taskExecutor.SetMetricsObserver(taskMetricsAdapter{registry: registry})
	pluginStack.renderer.SetMetricsObserver(renderMetricsAdapter{registry: registry})
	return registry, pluginservice.StartPluginRuntimeStateGaugeRefresh(registry, pluginStack.Plugins)
}
