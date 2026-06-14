package apphost

import (
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
)

func registerAppPublicRoutes(router chi.Router, deps httpServerDeps) {
	router.Get("/healthz", health.NewLivenessHandler())
	router.Get("/readyz", health.NewReadinessHandler(func() health.ReadinessReport {
		return deps.systemHandler.CurrentReadiness()
	}))
	router.Post("/api/setup/admin", deps.authHandler.HandleSetupAdmin())
	router.Get("/api/setup/status", deps.managementHandler.HandleSetupStatus())
	router.Post("/api/session/login", deps.authHandler.HandleSessionLogin())
	router.Get("/api/launcher/status", deps.managementHandler.HandleLauncherStatus())
	router.Post("/api/launcher/shutdown", deps.managementHandler.HandleLauncherShutdown())
	router.Get("/api/protocols/onebot11/reverse-ws", deps.protocolHandler.HandleProtocolOneBot11ReverseWS())
	router.Post("/api/protocols/onebot11/webhook", deps.protocolHandler.HandleProtocolOneBot11Webhook())
	if deps.pluginWebhooks != nil {
		deps.pluginWebhooks.RegisterPublicRoutes(router)
	}
	if deps.pluginManagementUI != nil {
		deps.pluginManagementUI.RegisterPublicRoutes(router)
	}
}

func registerAppProtectedRoutes(router chi.Router, deps httpServerDeps) {
	router.Delete("/api/session", deps.managementHandler.HandleSessionLogout())
	router.Get("/api/config", deps.configHandler.HandleConfigGet())
	router.Put("/api/config", deps.configHandler.HandleConfigPut())
	router.Get("/api/protocols/onebot11", deps.protocolHandler.HandleProtocolOneBot11Snapshot())
	router.Get("/api/protocols/onebot11/targets", deps.protocolHandler.HandleProtocolOneBot11Targets())
	router.Post("/api/protocols/onebot11/identities/resolve", deps.protocolHandler.HandleProtocolOneBot11IdentitiesResolve())
	router.Get("/api/protocols/onebot11/compatibility", deps.protocolHandler.HandleProtocolOneBot11Compatibility())
	if deps.governanceHandler != nil {
		deps.governanceHandler.RegisterProtectedRoutes(router)
	}
	router.Get("/api/logs", deps.logHandler.HandleLogsList())
	router.Get("/api/logs/{log_id}", deps.logHandler.HandleLogDetail())
	router.Get("/api/system/status", deps.managementHandler.HandleSystemStatus())
	router.Post("/api/system/shutdown", deps.managementHandler.HandleSystemShutdown())
	router.Post("/api/system/backup", deps.systemHandler.HandleSystemBackup())
	router.Post("/api/system/recovery/recheck", deps.systemHandler.HandleSystemRecoveryRecheck())
	router.Post("/api/system/recovery/confirm", deps.systemHandler.HandleSystemRecoveryConfirm())
	router.Post("/api/system/runtime/bootstrap", deps.systemHandler.HandleSystemRuntimeBootstrap())
	router.Get("/api/system/diagnostics/export", deps.systemHandler.HandleSystemDiagnosticsExport())
	if deps.metrics != nil {
		router.Get("/api/system/metrics", deps.metrics.HTTPHandler().ServeHTTP)
	}
	router.Get("/api/system/render/templates", deps.renderHandler.HandleSystemRenderTemplateList())
	router.Post("/api/system/render/templates/{template_id}/preview-html", deps.renderHandler.HandleSystemRenderTemplatePreviewHTML())
	router.Get("/api/system/render/templates/{template_id}/asset", deps.renderHandler.HandleSystemRenderTemplateAsset())
	router.Get("/api/system/render/templates/{template_id}", deps.renderHandler.HandleSystemRenderTemplateDetail())
	router.Get("/api/system/scheduler/jobs", deps.systemHandler.HandleSystemSchedulerJobList())
	router.Post("/api/system/scheduler/jobs/{job_id}/trigger", deps.systemHandler.HandleSystemSchedulerJobTrigger())
	router.Get("/api/third-party/accounts", deps.thirdPartyHandler.HandleThirdPartyAccountList())
	router.Put("/api/third-party/accounts/{platform}/{account_id}", deps.thirdPartyHandler.HandleThirdPartyAccountUpsert())
	router.Delete("/api/third-party/accounts/{platform}/{account_id}", deps.thirdPartyHandler.HandleThirdPartyAccountDelete())
	router.Get("/api/third-party/monitors", deps.thirdPartyHandler.HandleThirdPartyMonitorList())
	router.Get("/api/third-party/media", deps.thirdPartyHandler.HandleThirdPartyMedia())
	router.Post("/api/bilibili/login/qrcode", deps.bilibiliHandler.HandleBilibiliQRCodeLoginCreate())
	router.Get("/api/bilibili/login/qrcode/{login_id}", deps.bilibiliHandler.HandleBilibiliQRCodeLoginPoll())
	router.Get("/api/bilibili/users/resolve", deps.bilibiliHandler.HandleBilibiliUserResolve())
	router.Get("/api/bilibili/source/status", deps.bilibiliHandler.HandleBilibiliSourceStatus())
	router.Post("/api/bilibili/source/restart", deps.bilibiliHandler.HandleBilibiliSourceRestart())
	router.Get("/api/tasks", deps.taskHandler.HandleTaskList())
	router.Get("/api/tasks/{task_id}", deps.taskHandler.HandleTaskDetail())
	router.Post("/api/tasks/{task_id}/cancel", deps.taskHandler.HandleTaskCancel())
	if deps.pluginManagementUI != nil {
		deps.pluginManagementUI.RegisterProtectedRoutes(router)
	}
	router.Get("/ws/events", deps.eventsWS.HandleEventsWebSocket())
	router.Get("/ws/tasks", deps.tasksWS.HandleTasksWebSocket())
	router.Get("/ws/logs", deps.logsWS.HandleLogsWebSocket())
	router.Get("/ws/plugins/{id}/console", deps.consoleWS.HandlePluginConsoleWebSocket())
	managementhttp.RegisterPluginRoutes(router, deps.plugins, deps.tasks, deps.pluginRepository, deps.pluginInstaller, deps.pluginLifecycle, deps.pluginUninstaller, deps.grantRepository, currentPluginAutoGrantCapabilities(deps.state))
}

func currentPluginAutoGrantCapabilities(state *appRuntimeState) func() []string {
	return func() []string {
		if state == nil {
			return nil
		}
		return append([]string(nil), state.Config.Permission.AutoGrantCapabilities...)
	}
}
