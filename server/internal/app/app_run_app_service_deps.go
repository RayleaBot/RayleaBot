package app

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/bilibiliapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/taskapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/thirdpartyapi"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type configHTTPDeps struct {
	state            *appRuntimeState
	logs             *logging.Stream
	logRepository    logging.Repository
	renderer         *renderservice.Service
	pluginLogLimiter *localaction.PluginLogLimiter
	outboundLimiter  interface{ ApplyConfig(config.Config) }
	protocol         *protocolapi.Service
	eventIngress     *eventingress.Service
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
	configHandler      *configapi.Handlers
	authHandler        *authapi.Handlers
	managementHandler  *coreapi.Handlers
	governanceHandler  *governanceapi.Handlers
	taskHandler        *taskapi.Handlers
	logHandler         *logapi.Handlers
	renderHandler      *renderapi.Handlers
	systemHandler      *systemapi.Handlers
	protocolHandler    *protocolapi.Handlers
	thirdPartyHandler  *thirdpartyapi.ThirdPartyHandlers
	bilibiliHandler    *bilibiliapi.BilibiliHandlers
	eventsWS           *managementws.EventsHandler
	tasksWS            *managementws.TasksHandler
	logsWS             *managementws.LogsHandler
	consoleWS          *managementws.ConsoleHandler
	pluginWebhooks     *pluginwebhook.Service
	pluginManagementUI *pluginui.Handlers
	metrics            *metrics.Registry
}

func newConfigHTTPService(deps configHTTPDeps) *configruntime.Service {
	runtimeDeps := configruntime.Deps{
		CurrentConfig: func() config.Config {
			if deps.state == nil {
				return config.Config{}
			}
			return deps.state.Config
		},
		CurrentSummary: func() config.Summary {
			if deps.state == nil {
				return config.Summary{}
			}
			return deps.state.Summary
		},
		SetConfig: func(cfg config.Config) {
			if deps.state != nil {
				deps.state.Config = cfg
			}
		},
		SetSummary: func(summary config.Summary) {
			if deps.state != nil {
				deps.state.Summary = summary
			}
		},
		Logger:           runtimeStateLogger(deps.state),
		LogLevel:         runtimeStateLogLevel(deps.state),
		Logs:             deps.logs,
		LogRepository:    deps.logRepository,
		Renderer:         deps.renderer,
		PluginLogLimiter: deps.pluginLogLimiter,
		OutboundLimiter:  deps.outboundLimiter,
		EventIngress:     deps.eventIngress,
	}
	if deps.protocol != nil {
		runtimeDeps.Protocol = deps.protocol
	}
	return configruntime.NewService(runtimeDeps)
}

func runtimeStateLogger(state *appRuntimeState) *slog.Logger {
	if state == nil {
		return nil
	}
	return state.Logger
}

func runtimeStateLogLevel(state *appRuntimeState) *logging.LevelController {
	if state == nil {
		return nil
	}
	return state.LogLevel
}

func classifyConfigApplyEffects(oldCfg config.Config, newCfg config.Config) configapi.ApplyEffects {
	return configruntime.ClassifyApplyEffects(oldCfg, newCfg)
}

func configDocumentFromTyped(cfg config.Config) map[string]any {
	return configruntime.ConfigDocumentFromTyped(cfg)
}
