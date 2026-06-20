package servicegraph

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	thirdpartylogin "github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdpartylogin"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincapabilityview "github.com/RayleaBot/RayleaBot/server/internal/plugins/capabilityview"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type RuntimeState interface {
	CurrentConfig() config.Config
	CurrentSummary() config.Summary
	RepoRoot() string
	StartedAt() time.Time
	RuntimeLogger() *slog.Logger
	RedactString(string) string
}

type BuildDeps struct {
	Runtime               RuntimeState
	Platform              appplatform.State
	Plugins               pluginstack.State
	Events                eventstack.State
	Renderer              *renderservice.Service
	Metrics               *metrics.Registry
	Discovery             runtimepaths.PluginDiscoverySpec
	PluginValidator       *schema.Validator
	ManagementRedact      func(string) string
	BilibiliHTTPTransport http.RoundTripper
	BilibiliClock         func() time.Time
}

type Services struct {
	LocalActions      *localaction.Service
	PluginLifecycle   *pluginservice.Controller
	EventIngress      *eventingress.Service
	Protocol          *protocolapi.Service
	PluginWebhooks    *pluginwebhook.Service
	Governance        *governance.Service
	GovernanceEvents  *managementevents.GovernanceService
	Logs              *logging.ManagementService
	System            *systemsvc.Service
	ThirdParty        *thirdparty.Service
	ThirdPartyQRLogin *thirdpartylogin.Service
	BilibiliSource    *bilibilisource.Source
	BilibiliEvents    *managementevents.BilibiliSourceService
}

type BuildResult struct {
	Services              Services
	Runtimes              *runtimeregistry.Registry
	Status                *managementevents.ServiceStatusService
	BilibiliAccountClient *bilibilisession.AccountClient
	BilibiliQRLogin       *bilibilisession.QRLoginService
}

func Build(deps BuildDeps) (BuildResult, error) {
	runtimeState := deps.Runtime
	platform := deps.Platform
	pluginStack := deps.Plugins
	eventStack := deps.Events
	renderer := deps.Renderer
	logService := logging.NewManagementService(platform.Logs, platform.LogRepository)
	policyRepos := buildPolicyRepositories(platform)
	capabilityView := buildPluginCapabilityView(pluginStack, eventStack)
	governanceEvents := managementevents.NewGovernanceService()
	bilibiliEvents := managementevents.NewBilibiliSourceService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, policyRepos, governanceEvents)
	thirdPartyService, err := thirdparty.NewService(platform.Storage, platform.Secrets)
	if err != nil {
		return BuildResult{}, err
	}
	thirdPartyQRLogin := thirdpartylogin.NewService(deps.BilibiliHTTPTransport, deps.BilibiliClock)
	bilibiliSession := bilibilisession.NewSessionClient(deps.BilibiliHTTPTransport, deps.BilibiliClock, nil)
	localActions := buildLocalActionService(runtimeState, platform, pluginStack, eventStack, renderer, capabilityView, governanceService, thirdPartyService, bilibiliSession)
	configureLocalActionService(localActions, pluginStack, eventStack)
	runtimeRegistry := buildRuntimeRegistry(runtimeRegistryDeps{
		Logger:                     runtimeState.RuntimeLogger(),
		Console:                    platform.Console,
		RedactText:                 deps.ManagementRedact,
		StderrRateLimitBytesPerSec: runtimeState.CurrentConfig().Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         localActions.Execute,
	})
	systemService := systemsvc.New(systemsvc.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		CurrentSummary:   runtimeState.CurrentSummary,
		CurrentRepoRoot:  runtimeState.RepoRoot,
		CurrentStartedAt: runtimeState.StartedAt,
		Logger:           runtimeState.RuntimeLogger(),
		Auth:             platform.Auth,
		Adapter:          eventStack.Adapter,
		Plugins:          pluginStack.Plugins,
		Runtimes:         runtimeRegistry,
		Renderer:         renderer,
		Storage:          platform.Storage,
		PluginRepository: pluginStack.PluginRepository,
		TaskExecutor:     platform.TaskExecutor,
		LogRepository:    platform.LogRepository,
	})
	serviceStatusService := managementevents.NewServiceStatusService(systemService)
	systemService.SetStatusPublisher(serviceStatusService)
	lifecycle := buildPluginLifecycle(deps, platform, pluginStack, eventStack, renderer, runtimeRegistry, systemService)
	menuService := buildBuiltinMenuService(runtimeState, pluginStack, eventStack, renderer)
	eventIngress := eventingress.New(eventingress.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		Plugins:          pluginStack.Plugins,
		ReplyTargets:     eventStack.ReplyTargets,
		OutboundSender:   eventStack.OutboundSender,
		OutboundLimiter:  eventStack.OutboundLimiter,
		Renderer:         renderer,
		Menu:             menuService,
		Bridge:           eventStack.Bridge,
		Lifecycle:        lifecycle,
		MetadataEnricher: eventStack.Adapter,
		WhitelistRepo:    policyRepos.Whitelist,
		WhitelistState:   policyRepos.WhitelistState,
		BlacklistRepo:    policyRepos.Blacklist,
	})
	protocolService := protocolapi.NewService(runtimeState, eventStack.Adapter)
	pluginWebhooks := buildPluginWebhookGateway(runtimeState, platform, pluginStack, eventStack, lifecycle, capabilityView)
	pluginWebhooks.SetReplayMetrics(metrics.NewWebhookReplayObserver(deps.Metrics))
	localActions.SetWebhookGateway(pluginWebhooks)
	bilibiliSource, err := buildBilibiliSourceService(platform, pluginStack, eventStack, thirdPartyService, bilibiliSession, bilibiliEvents, deps)
	if err != nil {
		return BuildResult{}, err
	}

	return BuildResult{
		Services: Services{
			LocalActions:      localActions,
			PluginLifecycle:   lifecycle,
			EventIngress:      eventIngress,
			Protocol:          protocolService,
			PluginWebhooks:    pluginWebhooks,
			Governance:        governanceService,
			GovernanceEvents:  governanceEvents,
			Logs:              logService,
			System:            systemService,
			ThirdParty:        thirdPartyService,
			ThirdPartyQRLogin: thirdPartyQRLogin,
			BilibiliSource:    bilibiliSource,
			BilibiliEvents:    bilibiliEvents,
		},
		Runtimes:              runtimeRegistry,
		Status:                serviceStatusService,
		BilibiliAccountClient: bilibilisession.NewAccountClient(deps.BilibiliHTTPTransport, deps.BilibiliClock, nil),
		BilibiliQRLogin:       bilibilisession.NewQRLoginService(deps.BilibiliHTTPTransport, deps.BilibiliClock),
	}, nil
}

func buildPluginCapabilityView(pluginStack pluginstack.State, eventStack eventstack.State) *plugincapabilityview.View {
	capabilityView := plugincapabilityview.New(plugincapabilityview.Deps{
		Plugins: pluginStack.Plugins,
	})
	if eventStack.Dispatcher != nil {
		eventStack.Dispatcher.SetCapabilityChecker(capabilityView.CapabilityDeclared)
	}
	return capabilityView
}

func buildGovernanceService(runtimeState RuntimeState, pluginStack pluginstack.State, policy policyRepositories, events *managementevents.GovernanceService) *governance.Service {
	return governance.NewService(governance.Deps{
		CurrentConfig:  runtimeState.CurrentConfig,
		Plugins:        pluginStack.Plugins,
		BlacklistRepo:  policy.Blacklist,
		WhitelistRepo:  policy.Whitelist,
		WhitelistState: policy.WhitelistState,
		NotifyChanged:  events.PublishChanged,
	})
}
