package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi"
)

func registerProtectedRoutes(r chi.Router, deps Deps) {
	r.Delete("/api/session", deps.Management.HandleSessionLogout())
	r.Get("/api/config", deps.Config.HandleConfigGet())
	r.Put("/api/config", deps.Config.HandleConfigPut())
	r.Get("/api/protocols/onebot11", deps.Protocol.HandleProtocolOneBot11Snapshot())
	r.Get("/api/protocols/onebot11/targets", deps.Protocol.HandleProtocolOneBot11Targets())
	r.Post("/api/protocols/onebot11/identities/resolve", deps.Protocol.HandleProtocolOneBot11IdentitiesResolve())
	r.Get("/api/protocols/onebot11/compatibility", deps.Protocol.HandleProtocolOneBot11Compatibility())
	if deps.Governance != nil {
		deps.Governance.RegisterProtectedRoutes(r)
	}
	r.Get("/api/logs", deps.Logs.HandleLogsList())
	r.Get("/api/logs/{log_id}", deps.Logs.HandleLogDetail())
	r.Get("/api/system/status", deps.Management.HandleSystemStatus())
	r.Post("/api/system/shutdown", deps.Management.HandleSystemShutdown())
	r.Post("/api/system/backup", deps.System.HandleSystemBackup())
	r.Post("/api/system/recovery/recheck", deps.System.HandleSystemRecoveryRecheck())
	r.Post("/api/system/recovery/confirm", deps.System.HandleSystemRecoveryConfirm())
	r.Post("/api/system/runtime/bootstrap", deps.System.HandleSystemRuntimeBootstrap())
	r.Get("/api/system/diagnostics/export", deps.System.HandleSystemDiagnosticsExport())
	if deps.Metrics != nil {
		r.Get("/api/system/metrics", deps.Metrics.HTTPHandler().ServeHTTP)
	}
	r.Get("/api/system/render/templates", deps.Render.HandleSystemRenderTemplateList())
	r.Post("/api/system/render/templates/{template_id}/preview-html", deps.Render.HandleSystemRenderTemplatePreviewHTML())
	r.Get("/api/system/render/templates/{template_id}/asset", deps.Render.HandleSystemRenderTemplateAsset())
	r.Get("/api/system/render/templates/{template_id}", deps.Render.HandleSystemRenderTemplateDetail())
	r.Get("/api/system/scheduler/jobs", deps.System.HandleSystemSchedulerJobList())
	r.Post("/api/system/scheduler/jobs/{job_id}/trigger", deps.System.HandleSystemSchedulerJobTrigger())
	r.Get("/api/third-party/accounts", deps.ThirdParty.HandleThirdPartyAccountList())
	r.Put("/api/third-party/accounts/{platform}/{account_id}", deps.ThirdParty.HandleThirdPartyAccountUpsert())
	r.Delete("/api/third-party/accounts/{platform}/{account_id}", deps.ThirdParty.HandleThirdPartyAccountDelete())
	r.Get("/api/third-party/monitors", deps.ThirdParty.HandleThirdPartyMonitorList())
	r.Get("/api/third-party/media", deps.ThirdParty.HandleThirdPartyMedia())
	r.Post("/api/bilibili/login/qrcode", deps.Bilibili.HandleBilibiliQRCodeLoginCreate())
	r.Get("/api/bilibili/login/qrcode/{login_id}", deps.Bilibili.HandleBilibiliQRCodeLoginPoll())
	r.Get("/api/bilibili/users/resolve", deps.Bilibili.HandleBilibiliUserResolve())
	r.Get("/api/bilibili/source/status", deps.Bilibili.HandleBilibiliSourceStatus())
	r.Post("/api/bilibili/source/restart", deps.Bilibili.HandleBilibiliSourceRestart())
	r.Get("/api/tasks", deps.Tasks.HandleTaskList())
	r.Get("/api/tasks/{task_id}", deps.Tasks.HandleTaskDetail())
	r.Post("/api/tasks/{task_id}/cancel", deps.Tasks.HandleTaskCancel())
	if deps.PluginManagementUI != nil {
		deps.PluginManagementUI.RegisterProtectedRoutes(r)
	}
	r.Get("/ws/events", deps.EventsWS.HandleEventsWebSocket())
	r.Get("/ws/tasks", deps.TasksWS.HandleTasksWebSocket())
	r.Get("/ws/logs", deps.LogsWS.HandleLogsWebSocket())
	r.Get("/ws/plugins/{id}/console", deps.ConsoleWS.HandlePluginConsoleWebSocket())
	pluginapi.RegisterPluginRoutes(r, deps.PluginCatalog, deps.TaskRegistry, deps.PluginRepository, deps.PluginInstaller, deps.PluginLifecycle, deps.PluginUninstaller, deps.GrantRepository, deps.AutoGrantCapabilities)
}
