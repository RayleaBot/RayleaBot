package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func wireMetrics(platform appplatform.State, events eventstack.State, renderer *renderservice.Service, plugins pluginstack.State) (*metrics.Registry, func()) {
	registry := metrics.New()
	events.Bridge.SetMetricsObserver(metrics.NewBridgeObserver(registry))
	events.Dispatcher.SetMetricsObserver(metrics.NewDispatchObserver(registry))
	events.Adapter.SetMetricsObserver(metrics.NewAdapterObserver(registry))
	platform.TaskExecutor.SetMetricsObserver(metrics.NewTaskObserver(registry))
	renderer.SetMetricsObserver(metrics.NewRenderObserver(registry))
	return registry, pluginservice.StartPluginStateGaugeRefresh(registry, plugins.Plugins)
}
