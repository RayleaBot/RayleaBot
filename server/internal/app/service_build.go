package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

type runtimeStateView interface {
	CurrentConfig() config.Config
	CurrentSummary() config.Summary
	RepoRoot() string
	StartedAt() time.Time
	RuntimeLogger() *slog.Logger
	RedactString(string) string
}

type serviceBuildDeps struct {
	Runtime               runtimeStateView
	Platform              PlatformState
	Plugins               PluginStackState
	Events                EventState
	Renderer              *renderservice.Service
	Metrics               *MetricsRegistry
	Discovery             runtimepaths.PluginDiscoverySpec
	PluginValidator       *config.Validator
	ManagementRedact      func(string) string
	BilibiliHTTPTransport http.RoundTripper
	BilibiliClock         func() time.Time
}

type Services struct {
	LocalActions      *actions.Service
	PluginLifecycle   *pluginservice.Controller
	EventIngress      *chatpolicy.Ingress
	Protocol          *managementevents.ProtocolService
	PluginWebhooks    *pluginwebhook.Service
	Governance        *governance.Service
	GovernanceEvents  *managementevents.GovernanceService
	Logs              *logging.ManagementService
	System            *systemsvc.Service
	ThirdParty        *thirdparty.Service
	ThirdPartyQRLogin *thirdparty.QRLoginService
}

type serviceBuildResult struct {
	Services                   Services
	Runtimes                   *pluginruntime.Registry
	Status                     *managementevents.ServiceStatusService
	ThirdPartyAccountValidator *AccountValidator
}

func buildServices(deps serviceBuildDeps) (serviceBuildResult, error) {
	runtimeState := deps.Runtime
	platform := deps.Platform
	pluginStack := deps.Plugins
	eventStack := deps.Events
	renderer := deps.Renderer
	logService := logging.NewManagementService(platform.Logs, platform.LogRepository)
	policyRepos := buildPolicyRepositories(platform)
	governanceEvents := managementevents.NewGovernanceService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, policyRepos, governanceEvents)
	integrations, err := buildIntegrations(integrationDeps{
		Config:        runtimeState.CurrentConfig(),
		Platform:      platform,
		Renderer:      renderer,
		HTTPTransport: deps.BilibiliHTTPTransport,
		Clock:         deps.BilibiliClock,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	pluginRuntime := buildPluginRuntime(pluginRuntimeDeps{
		Runtime:          runtimeState,
		Platform:         platform,
		Plugins:          pluginStack,
		Events:           eventStack,
		Renderer:         renderer,
		Governance:       governanceService,
		ManagementRedact: deps.ManagementRedact,
		ThirdParty:       integrations.ThirdParty,
	})
	runtimeRegistry := pluginRuntime.Runtimes
	systemService := systemsvc.New(systemsvc.Deps{
		CurrentConfig:       runtimeState.CurrentConfig,
		CurrentSummary:      runtimeState.CurrentSummary,
		CurrentRepoRoot:     runtimeState.RepoRoot,
		CurrentStartedAt:    runtimeState.StartedAt,
		Logger:              runtimeState.RuntimeLogger(),
		Auth:                platform.Auth,
		Adapter:             eventStack.Adapter,
		Plugins:             pluginStack.Plugins,
		Runtimes:            runtimeRegistry,
		Renderer:            renderer,
		Storage:             platform.Storage,
		ThirdParty:          thirdPartyDiagnostics{service: integrations.ThirdParty},
		Scheduler:           schedulerDiagnostics{scheduler: platform.Scheduler},
		PluginRepository:    pluginStack.PluginRepository,
		TaskExecutor:        platform.TaskExecutor,
		LogRepository:       platform.LogRepository,
		ResolveDatabasePath: runtimepaths.ResolveDatabasePath,
	})
	serviceStatusService := managementevents.NewServiceStatusService(systemService)
	systemService.SetStatusPublisher(serviceStatusService)
	pluginServices := buildPluginServices(pluginServiceDeps{
		Runtime:       runtimeState,
		Platform:      platform,
		Plugins:       pluginStack,
		Events:        eventStack,
		Renderer:      renderer,
		System:        systemService,
		PluginRuntime: pluginRuntime,
		Metrics:       deps.Metrics,
	})
	eventIngress := chatpolicy.NewIngress(chatpolicy.IngressDeps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		Plugins:          pluginStack.Plugins,
		ReplyTargets:     eventStack.ReplyTargets,
		OutboundSender:   eventStack.OutboundSender,
		OutboundLimiter:  eventStack.OutboundLimiter,
		Menu:             pluginServices.Menu,
		Bridge:           eventStack.Bridge,
		Lifecycle:        pluginServices.PluginLifecycle,
		MetadataEnricher: eventStack.Adapter,
		WhitelistRepo:    policyRepos.Whitelist,
		WhitelistState:   policyRepos.WhitelistState,
		BlacklistRepo:    policyRepos.Blacklist,
	})
	protocolService := managementevents.NewProtocolService(runtimeState, eventStack.Adapter)
	return serviceBuildResult{
		Services: Services{
			LocalActions:      pluginRuntime.LocalActions,
			PluginLifecycle:   pluginServices.PluginLifecycle,
			EventIngress:      eventIngress,
			Protocol:          protocolService,
			PluginWebhooks:    pluginServices.PluginWebhooks,
			Governance:        governanceService,
			GovernanceEvents:  governanceEvents,
			Logs:              logService,
			System:            systemService,
			ThirdParty:        integrations.ThirdParty,
			ThirdPartyQRLogin: integrations.ThirdPartyQRLogin,
		},
		Runtimes:                   runtimeRegistry,
		Status:                     serviceStatusService,
		ThirdPartyAccountValidator: integrations.AccountValidator,
	}, nil
}

func buildGovernanceService(runtimeState runtimeStateView, pluginStack PluginStackState, policy policyRepositories, events *managementevents.GovernanceService) *governance.Service {
	return governance.NewService(governance.Deps{
		CurrentConfig:  runtimeState.CurrentConfig,
		Plugins:        pluginStack.Plugins,
		BlacklistRepo:  policy.Blacklist,
		WhitelistRepo:  policy.Whitelist,
		WhitelistState: policy.WhitelistState,
		NotifyChanged:  events.PublishChanged,
	})
}

type policyRepositories struct {
	Blacklist      permission.BlacklistRepository
	Whitelist      permission.WhitelistRepository
	WhitelistState permission.WhitelistStateRepository
}

func buildPolicyRepositories(platform PlatformState) policyRepositories {
	return policyRepositories{
		Blacklist:      permission.NewSQLiteBlacklistRepository(platform.Storage.Read, platform.Storage.Write),
		Whitelist:      permission.NewSQLiteWhitelistRepository(platform.Storage.Read, platform.Storage.Write),
		WhitelistState: permission.NewSQLiteWhitelistStateRepository(platform.Storage.Read, platform.Storage.Write),
	}
}
