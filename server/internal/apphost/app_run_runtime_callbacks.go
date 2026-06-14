package apphost

import (
	"context"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
)

func configureAppRuntimeCallbacks(application *App, schedulerTriggers *schedulerTriggerProxy) {
	systemService := application.services.system
	lifecycle := application.services.pluginLifecycle
	eventIngress := application.services.eventIngress
	protocolService := application.services.protocol

	systemService.BindShutdownFlag(&application.process.shuttingDown)
	systemService.RefreshRecoverySummary()
	schedulerTriggers.Set(lifecycle.HandleSchedulerTrigger)

	if installer, ok := application.pluginStack.PluginInstaller.(interface{ SetAfterSuccess(func(string) error) }); ok {
		installer.SetAfterSuccess(func(string) error {
			if err := pluginservice.SyncCatalogRenderTemplates(context.Background(), application.pluginStack.renderer, application.pluginStack.Plugins); err != nil {
				return err
			}
			systemService.ReconcileRecoverySummaryBestEffort("plugin.install")
			return nil
		})
	}
	if installer, ok := application.pluginStack.PluginInstaller.(interface {
		SetRenderTemplateValidator(func(plugins.Snapshot) error)
	}); ok {
		installer.SetRenderTemplateValidator(pluginservice.ValidatePluginRenderTemplates)
	}
	if uninstaller, ok := application.pluginStack.PluginUninstaller.(interface {
		SetStopPlugin(plugins.StopPluginFunc)
		SetAfterSuccess(func(string))
	}); ok {
		uninstaller.SetStopPlugin(lifecycle.StopAndResetPlugin)
		uninstaller.SetAfterSuccess(func(pluginID string) {
			if application.pluginStack.renderer != nil {
				_ = application.pluginStack.renderer.RemovePluginTemplates(context.Background(), pluginID)
			}
			_ = pluginservice.SyncCatalogRenderTemplates(context.Background(), application.pluginStack.renderer, application.pluginStack.Plugins)
			systemService.ReconcileRecoverySummaryBestEffort("plugin.uninstall")
		})
	}
	if application.runtimes != nil {
		application.runtimes.SetOnCrash(lifecycle.HandleCrash)
	}
	if application.pluginStack.Adapter != nil {
		application.pluginStack.Adapter.SetEventHandler(eventIngress.HandleAdapterEvent)
		application.pluginStack.Adapter.SetReadyHandler(eventIngress.HandleAdapterReady)
		application.pluginStack.Adapter.SetStateHandler(func(adaptershell.Snapshot) {
			systemService.PublishStatusSnapshot()
			protocolService.PublishSnapshot()
		})
	}
}
