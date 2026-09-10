package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/accountvalidation"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/wsevents"
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
	PluginSettings    *settings.Service
	PluginLifecycle   *pluginservice.Controller
	EventIngress      *chatpolicy.Ingress
	Protocol          *wsevents.ProtocolService
	PluginWebhooks    *pluginwebhook.Service
	Governance        *governance.Service
	GovernanceEvents  *wsevents.GovernanceService
	ThirdPartyEvents  *wsevents.ThirdPartyAccountService
	Logs              *logging.ManagementService
	System            *systemsvc.Service
	ThirdParty        *thirdparty.Service
	ThirdPartyQRLogin *thirdparty.QRLoginService
	AccountValidation *accountvalidation.Service
}

type serviceBuildResult struct {
	Services                   Services
	Runtimes                   *pluginruntime.Registry
	Status                     *wsevents.ServiceStatusService
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
	governanceEvents := wsevents.NewGovernanceService()
	thirdPartyEvents := wsevents.NewThirdPartyAccountService()
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
	// 具体指针装配到接口字段前先判 nil，避免 typed nil 接口绕过
	// 插件动作层的 ThirdPartyResolve == nil 判断。
	var thirdPartyResolve actions.ThirdPartyResolver
	if integrations.DouyinBrowser != nil {
		thirdPartyResolve = integrations.DouyinBrowser
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
		ThirdPartyResolve: thirdPartyResolve,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	runtimeRegistry := pluginRuntime.Runtimes
	var serviceStatusService *wsevents.ServiceStatusService
	// A concrete pointer assigned straight into the interface would hand the
	// service a typed nil that passes a nil check, so only real clients enter
	// the map the protocol surface reads.
	qqStatus := make(map[string]wsevents.QQOfficialAdapter, len(eventStack.QQOfficial))
	for id, client := range eventStack.QQOfficial {
		if client == nil {
			continue
		}
		qqStatus[id] = client
	}
	protocolService := wsevents.NewProtocolService(runtimeState, wsevents.ProtocolServiceAdapters{
		OneBot11:   eventStack.OneBotShells,
		QQOfficial: qqStatus,
	})
	var systemRenderer systemsvc.RendererState
	if renderer != nil {
		systemRenderer = renderer
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
		Renderer:         systemRenderer,
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
	serviceStatusService = wsevents.NewServiceStatusService(systemService)
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

func buildGovernanceService(runtimeState runtimeStateView, pluginStack PluginStackState, policy policyRepositories, events *wsevents.GovernanceService) *governance.Service {
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
	Blacklist      permission.EntryRepository
	Whitelist      permission.EntryRepository
	WhitelistState permission.WhitelistStateRepository
}

func buildPolicyRepositories(platform PlatformState) policyRepositories {
	return policyRepositories{
		Blacklist:      permission.NewSQLiteAccessListRepository(platform.Storage.Read, platform.Storage.Write, permission.ListBlacklist),
		Whitelist:      permission.NewSQLiteAccessListRepository(platform.Storage.Read, platform.Storage.Write, permission.ListWhitelist),
		WhitelistState: permission.NewSQLiteWhitelistStateRepository(platform.Storage.Read, platform.Storage.Write),
	}
}
