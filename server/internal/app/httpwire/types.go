package httpwire

import (
	"log/slog"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/bilibiliapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/taskapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/thirdpartyapi"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type RuntimeState interface {
	CurrentConfig() config.Config
	CurrentSummary() config.Summary
	SetConfig(config.Config)
	SetSummary(config.Summary)
	RuntimeLogger() *slog.Logger
	RuntimeLogLevel() *logging.LevelController
	RepoRoot() string
}

type ConfigDeps struct {
	Runtime          RuntimeState
	Logs             *logging.Stream
	LogRepository    logging.Repository
	Renderer         *renderservice.Service
	PluginLogLimiter *localaction.PluginLogLimiter
	OutboundLimiter  interface{ ApplyConfig(config.Config) }
	Protocol         *protocolapi.Service
	EventIngress     *eventingress.Service
}

type serverDeps struct {
	runtime            RuntimeState
	auth               *auth.Manager
	tasks              *tasks.Registry
	plugins            *plugincatalog.Catalog
	pluginInstaller    plugins.InstallCoordinator
	pluginUninstaller  plugins.UninstallCoordinator
	pluginRepository   plugins.DesiredStateRepository
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

type BuildDeps struct {
	Runtime               RuntimeState
	Platform              appplatform.State
	Plugins               pluginstack.State
	Events                eventstack.State
	Renderer              *renderservice.Service
	Services              servicegraph.Services
	Status                *managementevents.ServiceStatusService
	BilibiliAccountClient *bilibilisession.AccountClient
	BilibiliQRLogin       *bilibilisession.QRLoginService
	Metrics               *metrics.Registry
	BilibiliHTTPTransport http.RoundTripper
	RequestShutdown       func()
}

type State struct {
	Router   http.Handler
	Server   *http.Server
	Handlers Handlers
}

type Handlers struct {
	Auth       *authapi.Handlers
	Management *coreapi.Handlers
	Tasks      *taskapi.Handlers
	EventsWS   *managementws.EventsHandler
}

func NewConfigService(deps ConfigDeps) *configruntime.Service {
	runtimeDeps := configruntime.Deps{
		CurrentConfig: func() config.Config {
			if deps.Runtime == nil {
				return config.Config{}
			}
			return deps.Runtime.CurrentConfig()
		},
		CurrentSummary: func() config.Summary {
			if deps.Runtime == nil {
				return config.Summary{}
			}
			return deps.Runtime.CurrentSummary()
		},
		SetConfig: func(cfg config.Config) {
			if deps.Runtime != nil {
				deps.Runtime.SetConfig(cfg)
			}
		},
		SetSummary: func(summary config.Summary) {
			if deps.Runtime != nil {
				deps.Runtime.SetSummary(summary)
			}
		},
		Logger:           runtimeStateLogger(deps.Runtime),
		LogLevel:         runtimeStateLogLevel(deps.Runtime),
		Logs:             deps.Logs,
		LogRepository:    deps.LogRepository,
		Renderer:         deps.Renderer,
		PluginLogLimiter: deps.PluginLogLimiter,
		OutboundLimiter:  deps.OutboundLimiter,
		EventIngress:     deps.EventIngress,
	}
	if deps.Protocol != nil {
		runtimeDeps.Protocol = deps.Protocol
	}
	return configruntime.NewService(runtimeDeps)
}

func runtimeStateLogger(state RuntimeState) *slog.Logger {
	if state == nil {
		return nil
	}
	return state.RuntimeLogger()
}

func runtimeStateLogLevel(state RuntimeState) *logging.LevelController {
	if state == nil {
		return nil
	}
	return state.RuntimeLogLevel()
}

func ClassifyConfigApplyEffects(oldCfg config.Config, newCfg config.Config) configapi.ApplyEffects {
	return configruntime.ClassifyApplyEffects(oldCfg, newCfg)
}

func ConfigDocumentFromTyped(cfg config.Config) map[string]any {
	return configruntime.ConfigDocumentFromTyped(cfg)
}
