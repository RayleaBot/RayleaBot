package lifecycle

import (
	"context"
	"log/slog"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type RuntimeRegistry interface {
	Get(pluginID string) (*runtimemanager.Manager, bool)
	GetOrCreate(pluginID string) *runtimemanager.Manager
	NewDetached() *runtimemanager.Manager
	Replace(pluginID string, manager *runtimemanager.Manager) *runtimemanager.Manager
	Delete(pluginID string) *runtimemanager.Manager
}

type BotIdentitySource interface {
	CurrentBotID() string
}

type Deps struct {
	CurrentConfig       func() config.Config
	RepoRoot            string
	Logger              *slog.Logger
	Plugins             *plugincatalog.Catalog
	DesiredStateRepo    plugins.DesiredStateRepository
	Runtimes            RuntimeRegistry
	Dispatcher          *dispatch.Dispatcher
	Scheduler           *scheduler.Engine
	PluginConfig        pluginconfig.Repository
	Adapter             BotIdentitySource
	Webhooks            *pluginwebhook.Registry
	Tasks               *tasks.Registry
	OnRecoveryChange    func(string)
	RefreshManifest     func(context.Context, string) (plugins.Snapshot, error)
	SyncRenderTemplates func(context.Context) error
}

type Controller struct {
	currentConfig       func() config.Config
	repoRoot            string
	logger              *slog.Logger
	plugins             *plugincatalog.Catalog
	desiredStateRepo    plugins.DesiredStateRepository
	runtimes            RuntimeRegistry
	dispatcher          *dispatch.Dispatcher
	scheduler           *scheduler.Engine
	pluginConfig        pluginconfig.Repository
	adapter             BotIdentitySource
	webhooks            *pluginwebhook.Registry
	tasks               *tasks.Registry
	onRecoveryChange    func(string)
	refreshManifest     func(context.Context, string) (plugins.Snapshot, error)
	syncRenderTemplates func(context.Context) error

	identityMu       sync.Mutex
	identityByPlugin map[string]string
}

func NewController(deps Deps) *Controller {
	return &Controller{
		currentConfig:       deps.CurrentConfig,
		repoRoot:            deps.RepoRoot,
		logger:              deps.Logger,
		plugins:             deps.Plugins,
		desiredStateRepo:    deps.DesiredStateRepo,
		runtimes:            deps.Runtimes,
		dispatcher:          deps.Dispatcher,
		scheduler:           deps.Scheduler,
		pluginConfig:        deps.PluginConfig,
		adapter:             deps.Adapter,
		webhooks:            deps.Webhooks,
		tasks:               deps.Tasks,
		onRecoveryChange:    deps.OnRecoveryChange,
		refreshManifest:     deps.RefreshManifest,
		syncRenderTemplates: deps.SyncRenderTemplates,
	}
}

func (c *Controller) config() config.Config {
	if c == nil || c.currentConfig == nil {
		return config.Config{}
	}
	return c.currentConfig()
}
