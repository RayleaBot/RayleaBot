package app

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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
	return a.auth
}

func (a *App) SetAuthManager(manager *auth.Manager) {
	if a == nil {
		return
	}
	a.auth = manager
	if a.authHandler != nil {
		a.authHandler.auth = manager
	}
	if a.managementHandler != nil {
		a.managementHandler.auth = manager
	}
	if a.systemService != nil {
		a.systemService.auth = manager
	}
}

func (a *App) Bridge() *bridge.Bridge {
	if a == nil {
		return nil
	}
	return a.bridge
}

func (a *App) SetBridge(eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.bridge = eventBridge
	if a.eventIngress != nil {
		a.eventIngress.bridge = eventBridge
	}
	if a.eventsWS != nil {
		a.eventsWS.bridge = eventBridge
	}
}

func (a *App) Logs() *logging.Stream {
	if a == nil {
		return nil
	}
	return a.logs
}

func (a *App) SetLogRepository(repository logging.Repository) {
	if a == nil {
		return
	}
	a.logRepository = repository
	if a.logService != nil {
		a.logService.repository = repository
	}
	if a.systemService != nil {
		a.systemService.logRepository = repository
	}
}

func (a *App) Console() *console.Stream {
	if a == nil {
		return nil
	}
	return a.console
}

func (a *App) Tasks() *tasks.Registry {
	if a == nil {
		return nil
	}
	return a.tasks
}

func (a *App) Plugins() *plugins.Catalog {
	if a == nil {
		return nil
	}
	return a.plugins
}

func (a *App) Storage() *storage.Store {
	if a == nil {
		return nil
	}
	return a.storage
}

func (a *App) PluginInstaller() plugins.InstallCoordinator {
	if a == nil {
		return nil
	}
	return a.pluginInstaller
}

func (a *App) SetPluginInstaller(installer plugins.InstallCoordinator) {
	if a == nil {
		return
	}
	a.pluginInstaller = installer
	if a.taskHandler != nil {
		a.taskHandler.pluginInstaller = installer
	}
}
