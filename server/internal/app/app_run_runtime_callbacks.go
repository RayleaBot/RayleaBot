package app

import (
	"context"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
)

func configureAppRuntimeCallbacks(application *App, schedulerTriggers *schedulerTriggerProxy) {
	systemService := application.services.System
	lifecycle := application.services.PluginLifecycle
	eventIngress := application.services.EventIngress
	protocolService := application.services.Protocol

	systemService.BindShutdownFlag(&application.process.shuttingDown)
	systemService.RefreshRecoverySummary()
	schedulerTriggers.Set(lifecycle.HandleSchedulerTrigger)

	if installer, ok := application.pluginStack.PluginInstaller.(interface{ SetAfterSuccess(func(string) error) }); ok {
		installer.SetAfterSuccess(func(string) error {
			if err := renderplugintemplates.SyncCatalogRenderTemplates(context.Background(), application.renderStack.Renderer, application.pluginStack.Plugins); err != nil {
				return err
			}
			systemService.ReconcileRecoverySummaryBestEffort("plugin.install")
			return nil
		})
	}
	if installer, ok := application.pluginStack.PluginInstaller.(interface {
		SetRenderTemplateValidator(func(plugins.Snapshot) error)
	}); ok {
		installer.SetRenderTemplateValidator(renderplugintemplates.ValidatePluginRenderTemplates)
	}
	if uninstaller, ok := application.pluginStack.PluginUninstaller.(interface {
		SetStopPlugin(plugins.StopPluginFunc)
		SetAfterSuccess(func(string))
	}); ok {
		uninstaller.SetStopPlugin(lifecycle.StopAndResetPlugin)
		uninstaller.SetAfterSuccess(func(pluginID string) {
			if application.renderStack.Renderer != nil {
				_ = application.renderStack.Renderer.RemovePluginTemplates(context.Background(), pluginID)
			}
			_ = renderplugintemplates.SyncCatalogRenderTemplates(context.Background(), application.renderStack.Renderer, application.pluginStack.Plugins)
			systemService.ReconcileRecoverySummaryBestEffort("plugin.uninstall")
		})
	}
	if application.runtimes != nil {
		application.runtimes.SetOnCrash(lifecycle.HandleCrash)
	}
	if application.eventStack.Adapter != nil {
		application.eventStack.Adapter.SetEventHandler(eventIngress.HandleAdapterEvent)
		application.eventStack.Adapter.SetReadyHandler(eventIngress.HandleAdapterReady)
		application.eventStack.Adapter.SetStateHandler(func(adaptershell.Snapshot) {
			systemService.PublishStatusSnapshot()
			protocolService.PublishSnapshot()
		})
	}
}
