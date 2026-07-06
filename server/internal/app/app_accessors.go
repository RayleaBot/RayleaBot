package app

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
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
	return a.state.CurrentConfig()
}

func (a *App) AuthManager() *auth.Manager {
	if a == nil {
		return nil
	}
	return a.platform.Auth
}

func (a *App) Bridge() *bridge.Bridge {
	if a == nil {
		return nil
	}
	return a.eventStack.Bridge
}

func (a *App) HandleAdapterEvent(ctx context.Context, event onebot11.NormalizedEvent) {
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
