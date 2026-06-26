package servicegraph

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph/integrationmodule"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph/pluginmodule"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
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
	LocalActions      *pluginmodule.LocalActionService
	PluginLifecycle   *pluginmodule.LifecycleController
	EventIngress      *eventingress.Service
	Protocol          *protocolapi.Service
	PluginWebhooks    *pluginmodule.WebhookService
	Governance        *governance.Service
	GovernanceEvents  *managementevents.GovernanceService
	Logs              *logging.ManagementService
	System            *systemsvc.Service
	ThirdParty        *integrationmodule.ThirdPartyService
	ThirdPartyQRLogin *integrationmodule.ThirdPartyQRLoginService
}

type BuildResult struct {
	Services                   Services
	Runtimes                   *runtimeregistry.Registry
	Status                     *managementevents.ServiceStatusService
	ThirdPartyAccountValidator *integrationmodule.AccountValidator
}

func Build(deps BuildDeps) (BuildResult, error) {
	runtimeState := deps.Runtime
	platform := deps.Platform
	pluginStack := deps.Plugins
	eventStack := deps.Events
	renderer := deps.Renderer
	logService := logging.NewManagementService(platform.Logs, platform.LogRepository)
	policyRepos := buildPolicyRepositories(platform)
	governanceEvents := managementevents.NewGovernanceService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, policyRepos, governanceEvents)
	integrations, err := integrationmodule.Build(integrationmodule.Deps{
		Config:        runtimeState.CurrentConfig(),
		Platform:      platform,
		Renderer:      renderer,
		HTTPTransport: deps.BilibiliHTTPTransport,
		Clock:         deps.BilibiliClock,
	})
	if err != nil {
		return BuildResult{}, err
	}
	pluginRuntime := pluginmodule.BuildRuntime(pluginmodule.RuntimeDeps{
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
		ThirdParty:       thirdPartyDiagnostics{service: integrations.ThirdParty},
		Scheduler:        schedulerDiagnostics{scheduler: platform.Scheduler},
		PluginRepository: pluginStack.PluginRepository,
		TaskExecutor:     platform.TaskExecutor,
		LogRepository:    platform.LogRepository,
	})
	serviceStatusService := managementevents.NewServiceStatusService(systemService)
	systemService.SetStatusPublisher(serviceStatusService)
	pluginServices := pluginmodule.BuildServices(pluginmodule.ServiceDeps{
		Runtime:       runtimeState,
		Platform:      platform,
		Plugins:       pluginStack,
		Events:        eventStack,
		Renderer:      renderer,
		System:        systemService,
		PluginRuntime: pluginRuntime,
		Metrics:       deps.Metrics,
	})
	eventIngress := eventingress.New(eventingress.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		Plugins:          pluginStack.Plugins,
		ReplyTargets:     eventStack.ReplyTargets,
		OutboundSender:   eventStack.OutboundSender,
		OutboundLimiter:  eventStack.OutboundLimiter,
		Renderer:         renderer,
		Menu:             pluginServices.Menu,
		Bridge:           eventStack.Bridge,
		Lifecycle:        pluginServices.PluginLifecycle,
		MetadataEnricher: eventStack.Adapter,
		WhitelistRepo:    policyRepos.Whitelist,
		WhitelistState:   policyRepos.WhitelistState,
		BlacklistRepo:    policyRepos.Blacklist,
	})
	protocolService := protocolapi.NewService(runtimeState, eventStack.Adapter)
	return BuildResult{
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
