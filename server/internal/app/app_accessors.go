package app

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (a *App) Logger() *slog.Logger {
	if a == nil || a.state == nil {
		return nil
	}
	return a.state.Logger
}

func (a *App) CurrentConfig() config.Config {
	if a == nil || a.state == nil {
		return config.Config{}
	}
	return a.state.Config
}

func (a *App) AuthManager() *auth.Manager {
	if a == nil {
		return nil
	}
	return a.platform.Auth
}

func (a *App) SetAuthManager(manager *auth.Manager) {
	if a == nil {
		return
	}
	a.platform.Auth = manager
	if a.httpHandlers.Auth != nil {
		a.httpHandlers.Auth.SetAuthManager(manager)
	}
	if a.httpHandlers.Management != nil {
		a.httpHandlers.Management.SetAuthManager(manager)
	}
	if a.services.System != nil {
		a.services.System.SetAuth(manager)
	}
}

func (a *App) Bridge() *bridge.Bridge {
	if a == nil {
		return nil
	}
	return a.eventStack.Bridge
}

func (a *App) SetBridge(eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.eventStack.Bridge = eventBridge
	if a.services.EventIngress != nil {
		a.services.EventIngress.SetBridge(eventBridge)
	}
	if a.httpHandlers.EventsWS != nil {
		a.httpHandlers.EventsWS.SetBridge(eventBridge)
	}
}

func (a *App) HandleAdapterEvent(ctx context.Context, event adapterintake.NormalizedEvent) {
	if a == nil || a.services.EventIngress == nil {
		return
	}
	a.services.EventIngress.HandleAdapterEvent(ctx, event)
}

func (a *App) Logs() *logging.Stream {
	if a == nil {
		return nil
	}
	return a.platform.Logs
}

func (a *App) SetLogRepository(repository logging.Repository) {
	if a == nil {
		return
	}
	a.platform.LogRepository = repository
	if a.services.Logs != nil {
		a.services.Logs.SetRepository(repository)
	}
	if a.services.System != nil {
		a.services.System.SetLogRepository(repository)
	}
}

func (a *App) Console() *console.Stream {
	if a == nil {
		return nil
	}
	return a.platform.Console
}

func (a *App) Tasks() *tasks.Registry {
	if a == nil {
		return nil
	}
	return a.platform.Tasks
}

func (a *App) Plugins() *plugincatalog.Catalog {
	if a == nil {
		return nil
	}
	return a.pluginStack.Plugins
}

func (a *App) Storage() *storage.Store {
	if a == nil {
		return nil
	}
	return a.platform.Storage
}

func (a *App) PluginInstaller() plugins.InstallCoordinator {
	if a == nil {
		return nil
	}
	return a.pluginStack.PluginInstaller
}

func (a *App) SetPluginInstaller(installer plugins.InstallCoordinator) {
	if a == nil {
		return
	}
	a.pluginStack.PluginInstaller = installer
	if a.httpHandlers.Tasks != nil {
		a.httpHandlers.Tasks.SetPluginInstaller(installer)
	}
}
