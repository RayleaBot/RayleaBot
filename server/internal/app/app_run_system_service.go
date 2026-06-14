package app

import (
	"sync/atomic"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type systemService struct {
	state            *appRuntimeState
	auth             *auth.Manager
	adapter          *adapter.Shell
	plugins          *plugincatalog.Catalog
	runtimes         *runtimeRegistry
	renderer         *render.Service
	storage          *storage.Store
	pluginRepository plugins.DesiredStateRepository
	taskExecutor     *tasks.Executor
	logRepository    logging.Repository
	shuttingDown     *atomic.Bool
	statusPublisher  *managementevents.ServiceStatusService
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

func (s *systemService) SystemStatus() string {
	return s.systemStatus()
}

func (s *systemService) ManagementStatusSnapshot() managementhttp.SystemStatusResponse {
	return managementhttp.SystemStatusResponse{
		Status:          s.systemStatus(),
		AdapterState:    string(stateOrIdle(s.adapter.Snapshot().State)),
		ActivePlugins:   s.activePluginCount(),
		UptimeSeconds:   s.uptimeSeconds(),
		RecoverySummary: s.state.recoverySummarySnapshot(),
	}
}
