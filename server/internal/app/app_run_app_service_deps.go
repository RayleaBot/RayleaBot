package app

import (
	"context"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type eventMetadataEnricher interface {
	EnrichEventMetadata(context.Context, adapterintake.NormalizedEvent) adapterintake.NormalizedEvent
}

type eventIngressDeps struct {
	state            *appRuntimeState
	plugins          *plugincatalog.Catalog
	replyTargets     *replyTargetCache
	outboundSender   outboundActionSender
	outboundLimiter  outbound.MessageLimiter
	renderer         *renderservice.Service
	menu             *menuext.Service
	bridge           *bridge.Bridge
	lifecycle        *pluginservice.Controller
	metadataEnricher eventMetadataEnricher
	whitelistRepo    permission.WhitelistRepository
	whitelistState   permission.WhitelistStateRepository
	blacklistRepo    permission.BlacklistRepository
}

type systemServiceDeps struct {
	state            *appRuntimeState
	auth             *auth.Manager
	adapter          *adaptershell.Shell
	plugins          *plugincatalog.Catalog
	runtimes         *runtimeRegistry
	renderer         *renderservice.Service
	storage          *storage.Store
	pluginRepository plugins.DesiredStateRepository
	taskExecutor     *tasks.Executor
	logRepository    logging.Repository
}

type configHTTPDeps struct {
	state            *appRuntimeState
	logs             *logging.Stream
	logRepository    logging.Repository
	renderer         *renderservice.Service
	pluginLogLimiter *localaction.PluginLogLimiter
	outboundLimiter  interface{ ApplyConfig(config.Config) }
	protocol         *managementhttp.ProtocolService
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
	pluginLifecycle    *pluginservice.Controller
	renderer           *renderservice.Service
	configHandler      *managementhttp.ConfigHandlers
	authHandler        *managementhttp.AuthHandlers
	managementHandler  *managementhttp.ManagementHandlers
	governanceHandler  *governance.Handlers
	taskHandler        *managementhttp.TaskHandlers
	logHandler         *managementhttp.LogHandlers
	renderHandler      *managementhttp.RenderHandlers
	systemHandler      *managementhttp.SystemHandlers
	protocolHandler    *managementhttp.ProtocolHandlers
	thirdPartyHandler  *managementhttp.ThirdPartyHandlers
	bilibiliHandler    *managementhttp.BilibiliHandlers
	eventsWS           *managementws.EventsHandler
	tasksWS            *managementws.TasksHandler
	logsWS             *managementws.LogsHandler
	consoleWS          *managementws.ConsoleHandler
	pluginWebhooks     *pluginwebhook.Service
	pluginManagementUI *pluginui.Handlers
	metrics            *metrics.Registry
}
