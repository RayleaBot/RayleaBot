package app

import (
	"context"
	"sync"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type pluginLifecycleController struct {
	state               *appRuntimeState
	plugins             *plugincatalog.Catalog
	desiredStateRepo    plugins.DesiredStateRepository
	grants              *pluginGrantView
	runtimes            *runtimeRegistry
	dispatcher          *dispatch.Dispatcher
	scheduler           *scheduler.Engine
	pluginConfig        pluginconfig.Repository
	adapter             *adapter.Shell
	webhooks            *pluginwebhook.Registry
	tasks               *tasks.Registry
	onRecoveryChange    func(string)
	refreshManifest     func(context.Context, string) (plugins.Snapshot, error)
	syncRenderTemplates func(context.Context) error

	identityMu       sync.Mutex
	identityByPlugin map[string]string
}

func newPluginLifecycleController(deps pluginLifecycleDeps) *pluginLifecycleController {
	return &pluginLifecycleController{
		state:               deps.state,
		plugins:             deps.plugins,
		desiredStateRepo:    deps.desiredStateRepo,
		grants:              deps.grants,
		runtimes:            deps.runtimes,
		dispatcher:          deps.dispatcher,
		scheduler:           deps.scheduler,
		pluginConfig:        deps.pluginConfig,
		adapter:             deps.adapter,
		webhooks:            deps.webhooks,
		tasks:               deps.tasks,
		onRecoveryChange:    deps.onRecoveryChange,
		refreshManifest:     deps.refreshManifest,
		syncRenderTemplates: deps.syncRenderTemplates,
	}
}
