package router

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	authhttp "github.com/RayleaBot/RayleaBot/server/internal/management/authhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/management/bilibiliapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/pluginapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/taskapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/thirdpartyapi"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type RuntimeConfigSource interface {
	CurrentConfig() config.Config
}

type PluginManagementUIModule interface {
	PublicRouteModule
	ProtectedRouteModule
}

type ThirdPartyAccountValidator interface {
	CheckCookie(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error)
}

type ThirdPartyUserResolver interface {
	ResolveProfiles(context.Context, string, string, []map[string]string) ([]thirdparty.AccountProfile, bool, error)
}

type BuildDeps struct {
	RepoRoot               string
	ConfigSource           RuntimeConfigSource
	ConfigService          *configruntime.Service
	Auth                   *auth.Manager
	LoginFailures          *auth.LoginFailureTracker
	System                 *systemsvc.Service
	RequestShutdown        func()
	Governance             *governance.Service
	Tasks                  *tasks.Registry
	TaskExecutor           *tasks.Executor
	PluginCatalog          *plugincatalog.Catalog
	PluginInstaller        plugins.InstallCoordinator
	PluginUninstaller      plugins.UninstallCoordinator
	PluginRepository       plugins.DesiredStateRepository
	PluginLifecycle        *pluginservice.Controller
	Logs                   *logging.ManagementService
	Renderer               *renderservice.Service
	Scheduler              *scheduler.Engine
	Protocol               *protocolapi.Service
	ThirdParty             *thirdparty.Service
	ThirdPartyValidator    ThirdPartyAccountValidator
	ThirdPartyQRLogin      *common.Service
	ThirdPartyUserResolver ThirdPartyUserResolver
	BilibiliSource         *bilibili.Source
	BilibiliHTTPTransport  http.RoundTripper
	EventBridge            *bridge.Bridge
	ServiceStatus          *events.ServiceStatusService
	GovernanceEvents       *events.GovernanceService
	BilibiliEvents         *events.BilibiliSourceService
	Console                *console.Stream
	PluginWebhooks         PublicRouteModule
	PluginManagementUI     PluginManagementUIModule
	Metrics                *metrics.Registry
}

type BuildResult struct {
	RouterDeps Deps
	Handlers   Handlers
}

type Handlers struct {
	Auth       *authapi.Handlers
	Management *coreapi.Handlers
	Tasks      *taskapi.Handlers
	EventsWS   *managementws.EventsHandler
}

func RegisterBuilt(r chi.Router, deps BuildDeps) Handlers {
	modules := BuildModules(deps)
	Register(r, modules.RouterDeps, authhttp.RequireAuth(deps.Auth))
	return modules.Handlers
}

func BuildModules(deps BuildDeps) BuildResult {
	authHandler := authapi.NewHandlers(authapi.Deps{
		Config:        authConfigSource{source: deps.ConfigSource},
		Auth:          deps.Auth,
		LoginFailures: deps.LoginFailures,
	})
	managementHandler := coreapi.NewHandlers(coreapi.Deps{
		Auth:            deps.Auth,
		System:          deps.System,
		RequestShutdown: deps.RequestShutdown,
	})
	governanceHandler := governanceapi.NewHandlersWithService(deps.Governance)
	taskHandler := taskapi.NewHandlers(deps.Tasks, deps.TaskExecutor, deps.PluginInstaller)
	logHandler := logapi.NewHandlers(deps.Logs)
	renderHandler := renderapi.NewHandlers(deps.Renderer)
	systemHandler := systemapi.NewHandlers(deps.System, deps.Scheduler)
	protocolHandler := protocolapi.NewHandlers(deps.Protocol)
	thirdPartyHandler := thirdpartyapi.NewThirdPartyHandlers(
		deps.ThirdParty,
		deps.ThirdPartyValidator,
		deps.ThirdPartyQRLogin,
		deps.BilibiliSource,
		deps.BilibiliHTTPTransport,
		thirdpartyapi.WithUserResolver(deps.ThirdPartyUserResolver),
	)
	bilibiliHandler := bilibiliapi.NewBilibiliHandlers(deps.BilibiliSource, deps.ThirdPartyQRLogin, deps.BilibiliHTTPTransport)
	eventsWS := managementws.NewEventsHandler(deps.EventBridge, deps.PluginCatalog, deps.Protocol, deps.ServiceStatus, deps.GovernanceEvents, deps.BilibiliEvents)
	tasksWS := managementws.NewTasksHandler(deps.Tasks)
	logsWS := managementws.NewLogsHandler(deps.Logs)
	consoleWS := managementws.NewConsoleHandler(deps.Console, deps.PluginCatalog)
	configHandler := configapi.NewHandlers(deps.ConfigService)

	handlers := Handlers{
		Auth:       authHandler,
		Management: managementHandler,
		Tasks:      taskHandler,
		EventsWS:   eventsWS,
	}
	return BuildResult{
		Handlers: handlers,
		RouterDeps: Deps{
			RepoRoot: deps.RepoRoot,
			Readiness: func() health.ReadinessReport {
				return systemHandler.CurrentReadiness()
			},
			PublicRoutes: []PublicRouteModule{
				authHandler,
				managementHandler,
				protocolHandler,
				deps.PluginWebhooks,
				deps.PluginManagementUI,
			},
			ProtectedRoutes: []ProtectedRouteModule{
				managementHandler,
				configHandler,
				protocolHandler,
				governanceHandler,
				logHandler,
				systemapi.NewRoutes(systemHandler, deps.Metrics.HTTPHandler()),
				renderHandler,
				thirdPartyHandler,
				bilibiliHandler,
				taskHandler,
				deps.PluginManagementUI,
				ProtectedRouteFunc(func(r chi.Router) {
					r.Get("/ws/events", eventsWS.HandleEventsWebSocket())
					r.Get("/ws/tasks", tasksWS.HandleTasksWebSocket())
					r.Get("/ws/logs", logsWS.HandleLogsWebSocket())
					r.Get("/ws/plugins/{id}/console", consoleWS.HandlePluginConsoleWebSocket())
				}),
				pluginapi.RouteDeps{
					Catalog:      deps.PluginCatalog,
					TaskRegistry: deps.Tasks,
					Repository:   deps.PluginRepository,
					Installer:    deps.PluginInstaller,
					Uninstaller:  deps.PluginUninstaller,
					Lifecycle:    deps.PluginLifecycle,
				},
			},
		},
	}
}

type authConfigSource struct {
	source RuntimeConfigSource
}

func (s authConfigSource) AuthConfig() authapi.Config {
	if s.source == nil {
		return authapi.Config{}
	}
	cfg := s.source.CurrentConfig()
	return authapi.Config{
		SetupLocalOnly:     cfg.Web.SetupLocalOnly,
		LoginFailureLimit:  authapi.LoginFailureLimit(cfg),
		LoginFailureWindow: authapi.LoginFailureWindow(cfg),
	}
}
