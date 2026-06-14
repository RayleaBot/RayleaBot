package apphost

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
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
	protocol         *managementhttp.ProtocolService
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

func classifyConfigApplyEffects(oldCfg config.Config, newCfg config.Config) managementhttp.ConfigApplyEffects {
	return configruntime.ClassifyApplyEffects(oldCfg, newCfg)
}

func configDocumentFromTyped(cfg config.Config) map[string]any {
	return configruntime.ConfigDocumentFromTyped(cfg)
}
