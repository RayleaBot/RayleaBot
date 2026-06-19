package pluginstack

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	lifecyclemetrics "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle/metrics"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderbootstrap "github.com/RayleaBot/RayleaBot/server/internal/render/bootstrap"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const dispatcherRuntimeFlushInterval = 10 * time.Second

type Deps struct {
	Config       config.Config
	Logger       *slog.Logger
	Discovery    runtimepaths.PluginDiscoverySpec
	Validator    *schema.Validator
	Catalog      *plugincatalog.Catalog
	Tasks        *tasks.Registry
	Platform     appplatform.State
	RenderRunner renderbrowser.Runner
}

type State struct {
	Plugins           *plugincatalog.Catalog
	Adapter           *adaptershell.Shell
	Bridge            *bridge.Bridge
	Dispatcher        *dispatch.Dispatcher
	ReplyTargets      *outbound.ReplyTargetCache
	OutboundSender    eventingress.OutboundActionSender
	PluginInstaller   plugins.InstallCoordinator
	PluginUninstaller plugins.UninstallCoordinator
	PluginRepository  plugins.DesiredStateRepository
	PluginConfig      pluginconfig.Repository
	PluginFiles       *pluginfile.Service
	PluginKV          pluginkv.Repository
	GrantRepository   plugins.GrantRepository
	BlacklistRepo     permission.BlacklistRepository
	WhitelistRepo     permission.WhitelistRepository
	WhitelistState    permission.WhitelistStateRepository
	Webhooks          *pluginwebhook.Registry
	Renderer          *renderservice.Service
	PluginLogLimiter  *localaction.PluginLogLimiter
	OutboundLimiter   *outbound.MessageRateLimiter
}

func Build(deps Deps) (State, error) {
	adapterShell := adaptershell.New(deps.Config.OneBot, deps.Config.Adapter, deps.Logger)
	replyTargets := outbound.NewReplyTargetCache(outbound.DefaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(deps.Logger, adapterShell, replyTargets, deps.Config.Runtime.MaxPendingEventsPerPlugin)
	var cleanups []func()
	abort := func(err error) (State, error) {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
		_ = deps.Platform.Storage.Close()
		return State{}, err
	}
	outboundLimiter := outbound.NewMessageRateLimiter(deps.Config)
	eventDispatcher.SetOutboundLimiter(outboundLimiter)
	eventBridge := bridge.New(deps.Logger, eventDispatcher)
	eventBridge.SetAdapterStatsSource(adapterShell)
	eventBridge.SetDispatcherStatsSource(metrics.NewDispatcherStatsAdapter(eventDispatcher))
	eventDispatcher.SetRuntimePublisher(metrics.NewDispatcherRuntimePublisher(eventBridge))
	eventDispatcher.StartObservabilityFlush(dispatcherRuntimeFlushInterval)
	cleanups = append(cleanups, eventDispatcher.Close)

	pluginRepository, pluginKVRepository, pluginConfigRepository, err := buildPluginRepositories(deps.Platform)
	if err != nil {
		return abort(err)
	}
	webhookRegistry := pluginwebhook.NewRegistry()
	pluginFileService := pluginfile.NewService(filepath.Join(filepath.Dir(deps.Platform.Storage.Path), "plugins"))
	renderService, err := buildRenderService(deps)
	if err != nil {
		return abort(err)
	}
	cleanups = append(cleanups, func() { _ = renderService.Close() })
	blacklistRepo := permission.NewSQLiteBlacklistRepository(deps.Platform.Storage.Read, deps.Platform.Storage.Write)
	whitelistRepo := permission.NewSQLiteWhitelistRepository(deps.Platform.Storage.Read, deps.Platform.Storage.Write)
	whitelistStateRepo := permission.NewSQLiteWhitelistStateRepository(deps.Platform.Storage.Read, deps.Platform.Storage.Write)

	if err := hydratePluginCatalog(deps.Catalog, pluginRepository, pluginConfigRepository); err != nil {
		return abort(err)
	}
	runtimepaths.CleanupOrphanedInstallDirs(deps.Logger, deps.Discovery.Roots)
	if err := renderplugintemplates.SyncCatalogRenderTemplates(context.Background(), renderService, deps.Catalog); err != nil {
		return abort(err)
	}

	pluginInstallService, pluginUninstallService, err := buildPluginMutationServices(deps, pluginRepository)
	if err != nil {
		return abort(err)
	}

	return State{
		Plugins:           deps.Catalog,
		Adapter:           adapterShell,
		Bridge:            eventBridge,
		Dispatcher:        eventDispatcher,
		ReplyTargets:      replyTargets,
		OutboundSender:    adapterShell,
		PluginInstaller:   pluginInstallService,
		PluginUninstaller: pluginUninstallService,
		PluginRepository:  pluginRepository,
		PluginConfig:      pluginConfigRepository,
		PluginFiles:       pluginFileService,
		PluginKV:          pluginKVRepository,
		GrantRepository:   pluginRepository,
		BlacklistRepo:     blacklistRepo,
		WhitelistRepo:     whitelistRepo,
		WhitelistState:    whitelistStateRepo,
		Webhooks:          webhookRegistry,
		Renderer:          renderService,
		PluginLogLimiter:  localaction.NewPluginLogLimiter(deps.Config),
		OutboundLimiter:   outboundLimiter,
	}, nil
}

func buildRenderService(deps Deps) (*renderservice.Service, error) {
	renderBrowserPath := renderbootstrap.PrepareBrowserPath(context.Background(), deps.Logger, deps.Discovery.RepoRoot, deps.Config.Render.BrowserPath)
	renderService, err := renderservice.NewService(renderservice.Options{
		RepoRoot:           deps.Discovery.RepoRoot,
		OutputRoot:         filepath.Join(filepath.Dir(deps.Platform.Storage.Path), "render"),
		Store:              deps.Platform.Storage,
		Runner:             deps.RenderRunner,
		WorkerCount:        deps.Config.Render.WorkerCount,
		BrowserArgs:        deps.Config.Render.BrowserArgs,
		BrowserPath:        renderBrowserPath,
		QueueMaxLength:     deps.Config.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(deps.Config.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(deps.Config.Render.TimeoutSeconds) * time.Second,
		MaxRenderDataBytes: int(httpapi.MaxManagementJSONBodyBytes),
		FooterTemplate:     deps.Config.Render.FooterTemplate,
		DefaultOutput:      deps.Config.Render.DefaultOutput,
		DeviceScalePercent: deps.Config.Render.DeviceScalePercent,
		Logger:             deps.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create render service: %w", err)
	}
	return renderService, nil
}

type RuntimeRegistryDeps struct {
	Logger                     *slog.Logger
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	ExecuteLocalAction         runtimemanager.LocalActionExecutor
}

func BuildRuntimeRegistry(deps RuntimeRegistryDeps) *runtimeregistry.Registry {
	return runtimeregistry.New(deps.Logger, runtimemanager.Options{
		Console:                    deps.Console,
		RedactText:                 deps.RedactText,
		StderrRateLimitBytesPerSec: deps.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         deps.ExecuteLocalAction,
	})
}

type MenuDeps struct {
	CurrentConfig func() config.Config
	Logger        *slog.Logger
	Plugins       State
}

func BuildBuiltinMenuService(deps MenuDeps) *menuext.Service {
	return menuext.New(menuext.Deps{
		CurrentConfig: deps.CurrentConfig,
		Plugins:       deps.Plugins.Plugins,
		Renderer:      deps.Plugins.Renderer,
		Sender:        deps.Plugins.OutboundSender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if deps.Plugins.OutboundLimiter == nil {
				return nil
			}
			return deps.Plugins.OutboundLimiter.Wait(ctx, request)
		},
		Logger: deps.Logger,
	})
}

func WireMetrics(platform appplatform.State, pluginStack State) (*metrics.Registry, func()) {
	registry := metrics.New()
	pluginStack.Bridge.SetMetricsObserver(metrics.NewBridgeObserver(registry))
	pluginStack.Dispatcher.SetMetricsObserver(metrics.NewDispatchObserver(registry))
	pluginStack.Adapter.SetMetricsObserver(metrics.NewAdapterObserver(registry))
	platform.TaskExecutor.SetMetricsObserver(metrics.NewTaskObserver(registry))
	pluginStack.Renderer.SetMetricsObserver(metrics.NewRenderObserver(registry))
	return registry, lifecyclemetrics.StartPluginRuntimeStateGaugeRefresh(registry, pluginStack.Plugins)
}
