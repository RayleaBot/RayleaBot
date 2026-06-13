package app

import (
	"sync/atomic"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type systemService struct {
	state            *appRuntimeState
	auth             *auth.Manager
	adapter          *adapter.Shell
	plugins          *plugins.Catalog
	runtimes         *runtimeRegistry
	renderer         *render.Service
	storage          *storage.Store
	pluginRepository plugins.DesiredStateRepository
	taskExecutor     *tasks.Executor
	logRepository    logging.Repository
	shuttingDown     *atomic.Bool
	statusPublisher  *serviceStatusService
}

func newSystemService(deps systemServiceDeps) *systemService {
	return &systemService{
		state:            deps.state,
		auth:             deps.auth,
		adapter:          deps.adapter,
		plugins:          deps.plugins,
		runtimes:         deps.runtimes,
		renderer:         deps.renderer,
		storage:          deps.storage,
		pluginRepository: deps.pluginRepository,
		taskExecutor:     deps.taskExecutor,
		logRepository:    deps.logRepository,
	}
}
