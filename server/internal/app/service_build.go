package app

import (
	"log/slog"
	"net/http"
	"time"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	permissionsqlite "github.com/RayleaBot/RayleaBot/server/internal/bot/permission/sqlite"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/accountvalidation"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
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
	Renderer              *render.Service
	Metrics               *MetricsRegistry
	Discovery             plugincatalog.DiscoverySpec
	PluginValidator       *config.Validator
	ManagementRedact      func(string) string
	BilibiliHTTPTransport http.RoundTripper
	BilibiliClock         func() time.Time
}

type Services struct {
	LocalActions      *actions.Service
	PluginSettings    *settings.Service
	PluginLifecycle   *pluginservice.Controller
	EventIngress      *chatpolicy.Ingress
	Protocol          *adapterservice.Service
	PluginWebhooks    *pluginwebhook.Service
	Governance        *governance.Service
	GovernanceEvents  *managementevents.GovernanceService
	ThirdPartyEvents  *managementevents.ThirdPartyAccountService
	Logs              *logging.ManagementService
	System            *systemsvc.Service
	ThirdParty        *thirdparty.Service
	ThirdPartyQRLogin *thirdparty.QRLoginService
	AccountValidation *accountvalidation.Service
}

type serviceBuildResult struct {
	Services                   Services
	Runtimes                   *pluginruntime.Registry
	Status                     *managementevents.ServiceStatusService
	ThirdPartyAccountValidator *accountvalidation.Validator
}

type statusPublisherFunc func()

func (fn statusPublisherFunc) PublishSnapshot() {
	if fn != nil {
		fn()
	}
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
	thirdPartyEvents := managementevents.NewThirdPartyAccountService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, policyRepos, governanceEvents)
	integrations, err := buildIntegrations(integrationDeps{
		Config:               runtimeState.CurrentConfig(),
		Platform:             platform,
		Renderer:             renderer,
		HTTPTransport:        deps.BilibiliHTTPTransport,
		Clock:                deps.BilibiliClock,
		Logger:               runtimeState.RuntimeLogger(),
		RepoRoot:             runtimeState.RepoRoot(),
		NotifyAccountChanged: thirdPartyEvents.PublishChanged,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	pluginRuntime, err := buildPluginRuntime(pluginRuntimeDeps{
		Runtime:           runtimeState,
		Platform:          platform,
		Plugins:           pluginStack,
		Events:            eventStack,
		Renderer:          renderer,
		Governance:        governanceService,
		ManagementRedact:  deps.ManagementRedact,
		ThirdParty:        integrations.ThirdParty,
		AccountValidation: integrations.AccountValidation,
		ThirdPartyResolve: integrations.DouyinBrowser,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	runtimeRegistry := pluginRuntime.Runtimes
	var serviceStatusService *managementevents.ServiceStatusService
	qqStatus := make(map[string]adapterservice.QQOfficialAdapter, len(eventStack.QQOfficial))
	for id, client := range eventStack.QQOfficial {
		qqStatus[id] = client
	}
	protocolService, err := adapterservice.NewService(runtimeState, adapterservice.Instances{
		OneBot11:   eventStack.OneBotShells,
		QQOfficial: qqStatus,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	systemService, err := systemsvc.New(systemsvc.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		CurrentSummary:   runtimeState.CurrentSummary,
		CurrentRepoRoot:  runtimeState.RepoRoot,
		CurrentStartedAt: runtimeState.StartedAt,
		Logger:           runtimeState.RuntimeLogger(),
		Auth:             platform.Auth,
		Adapters:         protocolService,
		Plugins:          pluginStack.Plugins,
		Runtimes:         runtimeRegistry,
		Renderer:         renderer,
		Storage:          platform.Storage,
		ThirdParty:       thirdPartyDiagnostics{service: integrations.ThirdParty},
		Scheduler:        schedulerDiagnostics{scheduler: platform.Scheduler},
		PluginRepository: pluginStack.PluginRepository,
		TaskExecutor:     platform.TaskExecutor,
		LogRepository:    platform.LogRepository,
		StatusPublisher: statusPublisherFunc(func() {
			if serviceStatusService != nil {
				serviceStatusService.PublishSnapshot()
			}
		}),
		ResolveDatabasePath: runtimepaths.ResolveDatabasePath,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	serviceStatusService = managementevents.NewServiceStatusService(systemService)
	pluginServices, err := buildPluginServices(pluginServiceDeps{
		Runtime:       runtimeState,
		Platform:      platform,
		Plugins:       pluginStack,
		Events:        eventStack,
		Renderer:      renderer,
		System:        systemService,
		PluginRuntime: pluginRuntime,
		Metrics:       deps.Metrics,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
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
		MetadataEnricher: eventStack,
		WhitelistRepo:    policyRepos.Whitelist,
		WhitelistState:   policyRepos.WhitelistState,
		BlacklistRepo:    policyRepos.Blacklist,
	})
	return serviceBuildResult{
		Services: Services{
			LocalActions:      pluginRuntime.LocalActions,
			PluginSettings:    pluginRuntime.Settings,
			PluginLifecycle:   pluginServices.PluginLifecycle,
			EventIngress:      eventIngress,
			Protocol:          protocolService,
			PluginWebhooks:    pluginServices.PluginWebhooks,
			Governance:        governanceService,
			GovernanceEvents:  governanceEvents,
			ThirdPartyEvents:  thirdPartyEvents,
			Logs:              logService,
			System:            systemService,
			ThirdParty:        integrations.ThirdParty,
			ThirdPartyQRLogin: integrations.ThirdPartyQRLogin,
			AccountValidation: integrations.AccountValidation,
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
	Blacklist      governance.ManagementEntryRepository
	Whitelist      governance.ManagementEntryRepository
	WhitelistState permission.WhitelistStateRepository
}

func buildPolicyRepositories(platform PlatformState) policyRepositories {
	return policyRepositories{
		Blacklist:      permissionsqlite.NewAccessListRepository(platform.Storage.Read, platform.Storage.Write, permission.ListBlacklist),
		Whitelist:      permissionsqlite.NewAccessListRepository(platform.Storage.Read, platform.Storage.Write, permission.ListWhitelist),
		WhitelistState: permissionsqlite.NewWhitelistStateRepository(platform.Storage.Read, platform.Storage.Write),
	}
}
