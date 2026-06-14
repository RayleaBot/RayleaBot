package app

import (
	"context"
	"log/slog"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
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
	if a.httpHandlers.auth != nil {
		a.httpHandlers.auth.SetAuthManager(manager)
	}
	if a.httpHandlers.management != nil {
		a.httpHandlers.management.SetAuthManager(manager)
	}
	if a.services.system != nil {
		a.services.system.auth = manager
	}
}

func (a *App) Bridge() *bridge.Bridge {
	if a == nil {
		return nil
	}
	return a.pluginStack.Bridge
}

func (a *App) SetBridge(eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.pluginStack.Bridge = eventBridge
	if a.services.eventIngress != nil {
		a.services.eventIngress.bridge = eventBridge
	}
	if a.httpHandlers.eventsWS != nil {
		a.httpHandlers.eventsWS.SetBridge(eventBridge)
	}
}

func (a *App) HandleAdapterEvent(ctx context.Context, event adapterintake.NormalizedEvent) {
	if a == nil || a.services.eventIngress == nil {
		return
	}
	a.services.eventIngress.HandleAdapterEvent(ctx, event)
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
	if a.services.logs != nil {
		a.services.logs.repository = repository
	}
	if a.services.system != nil {
		a.services.system.logRepository = repository
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
	if a.httpHandlers.tasks != nil {
		a.httpHandlers.tasks.SetPluginInstaller(installer)
	}
}
