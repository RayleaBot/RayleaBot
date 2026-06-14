package apphost

import (
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
)

func wireAppMetrics(platform appPlatform, pluginStack appPlugins) (*metrics.Registry, func()) {
	registry := metrics.New()
	pluginStack.Bridge.SetMetricsObserver(metrics.NewBridgeObserver(registry))
	pluginStack.Dispatcher.SetMetricsObserver(metrics.NewDispatchObserver(registry))
	pluginStack.Adapter.SetMetricsObserver(metrics.NewAdapterObserver(registry))
	platform.taskExecutor.SetMetricsObserver(metrics.NewTaskObserver(registry))
	pluginStack.renderer.SetMetricsObserver(metrics.NewRenderObserver(registry))
	return registry, pluginservice.StartPluginRuntimeStateGaugeRefresh(registry, pluginStack.Plugins)
}
