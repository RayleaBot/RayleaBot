package system

import (
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

type App struct {
	state    *testAppState
	platform struct {
		Tasks        *tasks.Registry
		taskExecutor *tasks.Executor
	}
	pluginStack struct {
		Plugins  *plugincatalog.Catalog
		renderer *render.Service
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

func (a *App) setTestSystem(taskRegistry *tasks.Registry, taskExecutor *tasks.Executor, rendererService *render.Service, logRepository any) {
	if a == nil {
		return
	}
	a.platform.Tasks = taskRegistry
	a.platform.taskExecutor = taskExecutor
	a.pluginStack.renderer = rendererService
	var renderer RendererState
	if rendererService != nil {
		renderer = rendererService
	}
	service, err := New(Deps{
		CurrentConfig:    a.state.CurrentConfig,
		CurrentSummary:   func() config.Summary { return a.state.Summary },
		CurrentRepoRoot:  func() string { return a.state.repoRoot },
		CurrentStartedAt: func() time.Time { return a.state.startedAt },
		Logger:           a.state.Logger,
		Plugins:          a.pluginStack.Plugins,
		PluginRepository: &testutil.DesiredStateRecorder{},
		Renderer:         renderer,
		TaskExecutor:     taskExecutor,
		LogRepository:    nil,
	})
	if err != nil {
		panic(err)
	}
	a.services.system = service
}
