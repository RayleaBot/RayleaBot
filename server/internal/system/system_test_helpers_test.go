package system

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type App struct {
	state    *testAppState
	platform struct {
		Tasks        *tasks.Registry
		taskExecutor *tasks.Executor
	}
	pluginStack struct {
		Plugins  *plugincatalog.Catalog
		renderer *renderservice.Service
	}
	services struct {
		system *Service
	}
}

type testAppState struct {
	Config    config.Config
	Summary   config.Summary
	repoRoot  string
	startedAt time.Time
	Logger    *slog.Logger
}

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

func (s *testAppState) CurrentConfig() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.Config
}

func newTestAppState(cfg config.Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	app := &App{
		state: &testAppState{
			Config:    cfg,
			Logger:    logger,
			startedAt: time.Now().UTC(),
		},
	}
	app.pluginStack.Plugins = plugincatalog.New(nil)
	app.setTestSystem(nil, nil, nil, nil)
	return app
}

func (a *App) setTestSystem(taskRegistry *tasks.Registry, taskExecutor *tasks.Executor, rendererService *renderservice.Service, logRepository any) {
	if a == nil {
		return
	}
	a.platform.Tasks = taskRegistry
	a.platform.taskExecutor = taskExecutor
	a.pluginStack.renderer = rendererService
	a.services.system = New(Deps{
		CurrentConfig:    a.state.CurrentConfig,
		CurrentSummary:   func() config.Summary { return a.state.Summary },
		CurrentRepoRoot:  func() string { return a.state.repoRoot },
		CurrentStartedAt: func() time.Time { return a.state.startedAt },
		Logger:           a.state.Logger,
		Plugins:          a.pluginStack.Plugins,
		Renderer:         rendererService,
		TaskExecutor:     taskExecutor,
		LogRepository:    nil,
	})
}

func (a *App) autoPrepareRuntimeEnvironments(ctx context.Context) {
	a.services.system.AutoPrepareRuntimeEnvironments(ctx)
}

func (a *App) startupRuntimeState(kind string) (StartupRuntimeState, bool) {
	return a.services.system.StartupRuntimeState(kind)
}

func (a *App) setStartupRuntimeState(kind string, phase StartupRuntimePhase, issue *recovery.CompatibilityIssue) {
	a.services.system.SetStartupRuntimeState(kind, phase, issue)
}

func (a *App) managedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	return a.services.system.ManagedRuntimeDiagnostics(pluginsList)
}

func (a *App) handleSystemRecoveryRecheck() http.HandlerFunc {
	return managementhttp.NewSystemHandlers(a.services.system).HandleSystemRecoveryRecheck()
}

func (a *App) handleSystemRecoveryConfirm() http.HandlerFunc {
	return managementhttp.NewSystemHandlers(a.services.system).HandleSystemRecoveryConfirm()
}

func (a *App) handleSystemRuntimeBootstrap() http.HandlerFunc {
	return managementhttp.NewSystemHandlers(a.services.system).HandleSystemRuntimeBootstrap()
}
