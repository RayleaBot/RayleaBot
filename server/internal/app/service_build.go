package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/qqofficial"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/messagestats"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	permissionsqlite "github.com/RayleaBot/RayleaBot/server/internal/bot/permission/sqlite"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/browser"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
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
	Runtime            runtimeStateView
	Platform           PlatformState
	Plugins            PluginStackState
	Events             EventState
	Renderer           *render.Service
	ManagementRedact   func(string) string
	MessageStatsEvents *managementevents.MessageStatsService
}

type Services struct {
	LocalActions       *actions.Service
	PluginSettings     *settings.Service
	PluginLifecycle    *pluginservice.Controller
	EventIngress       *chatpolicy.Ingress
	Protocol           *adapterservice.Service
	PluginWebhooks     *pluginwebhook.Service
	Governance         *governance.Service
	GovernanceEvents   *managementevents.GovernanceService
	MessageStatsEvents *managementevents.MessageStatsService
	Logs               *logging.ManagementService
	System             *systemsvc.Service
	Browser            *browser.Manager
}

type serviceBuildResult struct {
	Services Services
	Runtimes *pluginruntime.Registry
	Status   *managementevents.ServiceStatusService
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
	if platform.MessageStats == nil {
		return serviceBuildResult{}, errors.New("message statistics service is required")
	}
	if deps.MessageStatsEvents == nil {
		return serviceBuildResult{}, errors.New("message statistics event service is required")
	}
	pluginStack := deps.Plugins
	eventStack := deps.Events
	renderer := deps.Renderer
	logService := logging.NewManagementService(platform.Logs, platform.LogRepository)
	policyRepos := buildPolicyRepositories(platform)
	governanceEvents := managementevents.NewGovernanceService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, policyRepos, governanceEvents)
	browserManager := buildBrowserManager(browserWiringDeps{
		Config:   runtimeState.CurrentConfig(),
		Renderer: renderer,
		Logger:   runtimeState.RuntimeLogger(),
		RepoRoot: runtimeState.RepoRoot(),
	})
	pluginRuntime, err := buildPluginRuntime(pluginRuntimeDeps{
		Runtime:          runtimeState,
		Platform:         platform,
		Plugins:          pluginStack,
		Events:           eventStack,
		Renderer:         renderer,
		Governance:       governanceService,
		ManagementRedact: deps.ManagementRedact,
		Browser:          browserManager,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	runtimeRegistry := pluginRuntime.Runtimes
	var serviceStatusService *managementevents.ServiceStatusService
	var eventIngress *chatpolicy.Ingress
	var publishAdapterState func()
	var publishRuntimeState func()
	protocolService, err := adapterservice.NewService(runtimeState, adapterservice.Instances{
		Observe: func(view adapterservice.AdaptersView) {
			items := make([]messagestats.Adapter, 0, len(view.Adapters))
			for _, a := range view.Adapters {
				items = append(items, messagestats.Adapter{ID: a.ID, Enabled: a.Enabled, Connected: a.State == "connected"})
			}
			platform.MessageStats.ObserveAdapters(items)
		},
		Published: func() { publishRuntimeState() },
		Reloaded:  platform.MessageStats.ReloadAdapter,
		Registry:  eventStack.Adapters,
		NewOneBot11: func(id string, settings config.OneBotConfig, adapter config.AdapterConfig) *onebot11.Shell {
			shell := onebot11.New(id, settings, adapter, runtimeState.RuntimeLogger())
			configureOneBotRuntime(shell, eventIngress, publishAdapterState)
			return shell
		},
		NewQQOfficial: func(id string, settings config.QQOfficialConfig, adapter config.AdapterConfig) adapterservice.QQOfficialAdapter {
			client := qqofficial.New(id, settings, adapter, runtimeState.RuntimeLogger())
			configureQQRuntime(client, eventIngress, publishAdapterState)
			return client
		},
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	pluginServices, err := buildPluginServices(pluginServiceDeps{
		Runtime:       runtimeState,
		Platform:      platform,
		Plugins:       pluginStack,
		Events:        eventStack,
		Renderer:      renderer,
		PluginRuntime: pluginRuntime,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	systemService, err := systemsvc.New(systemsvc.Deps{
		ListenHost:       runtimeState.CurrentConfig().Server.Host,
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
		Scheduler:        schedulerDiagnostics{scheduler: platform.Scheduler},
		TaskExecutor:     platform.TaskExecutor,
		LogRepository:    platform.LogRepository,
		StatusPublisher: statusPublisherFunc(func() {
			if serviceStatusService != nil {
				serviceStatusService.PublishSnapshot()
			}
		}),
		ResolveDatabasePath: runtimepaths.ResolveDatabasePath,
		RefreshPluginTools:  pluginServices.PluginLifecycle.RefreshManagedRuntimeEnvironment,
	})
	if err != nil {
		return serviceBuildResult{}, err
	}
	serviceStatusService = managementevents.NewServiceStatusService(systemService)
	eventIngress = chatpolicy.NewIngress(chatpolicy.IngressDeps{
		MessageReceived:  platform.MessageStats.Received,
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
		Conversations:    eventStack.Conversations,
	})
	publishAdapterState = protocolService.PublishSnapshot
	publishRuntimeState = func() {
		systemService.PublishStatusSnapshot()
		pluginServices.PluginLifecycle.SyncBotIdentities(context.Background())
	}
	initial := eventStack.Adapters.Snapshot()
	for _, instance := range initial.Config().Adapters {
		if shell := initial.OneBot11(instance.ID); shell != nil {
			configureOneBotRuntime(shell, eventIngress, publishAdapterState)
		}
		if client, ok := initial.QQOfficial(instance.ID).(*qqofficial.Client); ok {
			configureQQRuntime(client, eventIngress, publishAdapterState)
		}
	}

	return serviceBuildResult{
		Services: Services{
			LocalActions:       pluginRuntime.LocalActions,
			PluginSettings:     pluginRuntime.Settings,
			PluginLifecycle:    pluginServices.PluginLifecycle,
			EventIngress:       eventIngress,
			Protocol:           protocolService,
			PluginWebhooks:     pluginServices.PluginWebhooks,
			Governance:         governanceService,
			GovernanceEvents:   governanceEvents,
			MessageStatsEvents: deps.MessageStatsEvents,
			Logs:               logService,
			System:             systemService,
			Browser:            browserManager,
		},
		Runtimes: runtimeRegistry,
		Status:   serviceStatusService,
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

type schedulerDiagnostics struct {
	scheduler *scheduler.Engine
}

func (d schedulerDiagnostics) Timezone() string {
	return d.scheduler.Timezone()
}

func (d schedulerDiagnostics) DiagnosticsScheduler() systemsvc.DiagnosticsScheduler {
	result := systemsvc.DiagnosticsScheduler{}
	if d.scheduler == nil {
		return result
	}
	now := time.Now().UTC()
	result.Running = d.scheduler.RunningCount()
	for _, job := range d.scheduler.Jobs() {
		result.Total++
		if job.Enabled {
			result.Enabled++
			if !job.NextRun.After(now) {
				result.Pending++
			}
		} else {
			result.Disabled++
		}
		if job.LastError != nil {
			result.Failed++
		}
	}
	return result
}

func configureOneBotRuntime(shell *onebot11.Shell, ingress *chatpolicy.Ingress, publish func()) {
	shell.SetEventHandler(ingress.EnqueueAdapterEvent)
	shell.SetReadyHandler(ingress.HandleAdapterReady)
	shell.SetStateHandler(func(onebot11.Snapshot) { publish() })
}

func configureQQRuntime(client *qqofficial.Client, ingress *chatpolicy.Ingress, publish func()) {
	client.SetEventHandler(ingress.EnqueueAdapterEvent)
	client.SetReadyHandler(ingress.HandleAdapterReady)
	client.SetStateHandler(publish)
}
