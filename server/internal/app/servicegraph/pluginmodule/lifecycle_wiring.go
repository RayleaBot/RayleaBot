package pluginmodule

import pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"

func buildPluginLifecycle(deps ServiceDeps) *pluginservice.Controller {
	return pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:       deps.Runtime.CurrentConfig,
		RepoRoot:            deps.Runtime.RepoRoot(),
		Logger:              deps.Runtime.RuntimeLogger(),
		Plugins:             deps.Plugins.Plugins,
		DesiredStateRepo:    deps.Plugins.PluginRepository,
		Runtimes:            deps.PluginRuntime.Runtimes,
		Dispatcher:          deps.Events.Dispatcher,
		Scheduler:           deps.Platform.Scheduler,
		PluginConfig:        deps.Plugins.PluginConfig,
		Adapter:             deps.Events.Adapter,
		Webhooks:            deps.Plugins.Webhooks,
		Tasks:               deps.Platform.Tasks,
		OnRecoveryChange:    deps.System.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:     deps.Plugins.RefreshManifest,
		SyncRenderTemplates: pluginRenderTemplateSync(deps),
	})
}
