package app

import (
	"context"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type pluginLifecycleDeps struct {
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
}

type eventMetadataEnricher interface {
	EnrichEventMetadata(context.Context, adapter.NormalizedEvent) adapter.NormalizedEvent
}

type eventIngressDeps struct {
	state            *appRuntimeState
	plugins          *plugincatalog.Catalog
	replyTargets     *replyTargetCache
	outboundSender   outboundActionSender
	outboundLimiter  outbound.MessageLimiter
	renderer         *render.Service
	menu             *menuext.Service
	bridge           *bridge.Bridge
	lifecycle        *pluginLifecycleController
	metadataEnricher eventMetadataEnricher
	whitelistRepo    permission.WhitelistRepository
	whitelistState   permission.WhitelistStateRepository
	blacklistRepo    permission.BlacklistRepository
}

type systemServiceDeps struct {
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
}

type authHTTPDeps struct {
	config        authHTTPConfigSource
	auth          authSessionService
	loginFailures loginFailureRecorder
}

type managementHTTPDeps struct {
	auth            managementAuthService
	system          managementSystemService
	requestShutdown func()
}

type configHTTPDeps struct {
	state            *appRuntimeState
	logs             *logging.Stream
	logRepository    logging.Repository
	renderer         *render.Service
	pluginLogLimiter *localaction.PluginLogLimiter
	outboundLimiter  interface{ ApplyConfig(config.Config) }
	protocol         *protocolService
	eventIngress     *eventIngressService
	blacklistRepo    permission.BlacklistRepository
}

type httpServerDeps struct {
	state              *appRuntimeState
	auth               *auth.Manager
	tasks              *tasks.Registry
	plugins            *plugincatalog.Catalog
	pluginInstaller    plugins.InstallCoordinator
	pluginUninstaller  plugins.UninstallCoordinator
	pluginRepository   plugins.DesiredStateRepository
	grantRepository    plugins.GrantRepository
	pluginLifecycle    *pluginLifecycleController
	renderer           *render.Service
	configHandler      *configHTTPHandlers
	authHandler        *authHTTPHandlers
	managementHandler  *managementHTTPHandlers
	governanceHandler  *governance.Handlers
	taskHandler        *taskHTTPHandlers
	logHandler         *logHTTPHandlers
	renderHandler      *renderHTTPHandlers
	systemHandler      *systemHTTPHandlers
	protocolHandler    *protocolHTTPHandlers
	thirdPartyHandler  *thirdPartyHTTPHandlers
	bilibiliHandler    *bilibiliSourceHTTPHandlers
	eventsWS           *eventsWSHandler
	tasksWS            *tasksWSHandler
	logsWS             *logsWSHandler
	consoleWS          *consoleWSHandler
	pluginWebhooks     *pluginwebhook.Service
	pluginManagementUI *pluginui.Handlers
	metrics            *metrics.Registry
}
