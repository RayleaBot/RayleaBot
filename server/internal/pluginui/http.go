package pluginui

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type Deps struct {
	Plugins            *plugins.Catalog
	PluginConfig       pluginconfig.Repository
	Secrets            secrets.Store
	NotifyConfigChange func(context.Context, string)
	RefreshCommands    func(context.Context, string, map[string]any)
}

type Handlers struct {
	plugins            *plugins.Catalog
	pluginConfig       pluginconfig.Repository
	secrets            secrets.Store
	notifyConfigChange func(context.Context, string)
	refreshCommands    func(context.Context, string, map[string]any)
}

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{
		plugins:            deps.Plugins,
		pluginConfig:       deps.PluginConfig,
		secrets:            deps.Secrets,
		notifyConfigChange: deps.NotifyConfigChange,
		refreshCommands:    deps.RefreshCommands,
	}
}

func (h *Handlers) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
	router.Head("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
}

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsGet())
	router.Put("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsPut())
	router.Get("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsGet())
	router.Put("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsPut())
}
