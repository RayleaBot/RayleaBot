package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
)

func registerPublicRoutes(r chi.Router, deps Deps) {
	r.Get("/healthz", health.NewLivenessHandler())
	r.Get("/readyz", health.NewReadinessHandler(deps.Readiness))
	r.Post("/api/setup/admin", deps.Auth.HandleSetupAdmin())
	r.Get("/api/setup/status", deps.Management.HandleSetupStatus())
	r.Post("/api/session/login", deps.Auth.HandleSessionLogin())
	r.Get("/api/launcher/status", deps.Management.HandleLauncherStatus())
	r.Post("/api/launcher/shutdown", deps.Management.HandleLauncherShutdown())
	r.Get("/api/protocols/onebot11/reverse-ws", deps.Protocol.HandleProtocolOneBot11ReverseWS())
	r.Post("/api/protocols/onebot11/webhook", deps.Protocol.HandleProtocolOneBot11Webhook())
	if deps.PluginWebhooks != nil {
		deps.PluginWebhooks.RegisterPublicRoutes(r)
	}
	if deps.PluginManagementUI != nil {
		deps.PluginManagementUI.RegisterPublicRoutes(r)
	}
}
