package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

// dispatcherRuntimeFlushInterval bounds how often the dispatcher emits a
// dispatcher_runtime observability snapshot. The dispatcher publishes one
// frame per window even when counts are zero so subscribers can show
// liveness; the window stays short enough for a management dashboard to
// stay responsive but long enough to dampen drop bursts.
const dispatcherRuntimeFlushInterval = 10 * time.Second

func buildAppPlugins(
	state appBuildState,
	platform appPlatform,
	renderRunner render.Runner,
) (appPlugins, error) {
	adapterShell := adapter.New(state.core.Config.OneBot, state.core.Logger)
	replyTargets := newReplyTargetCache(defaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(state.core.Logger, adapterShell, replyTargets, state.core.Config.Runtime.MaxPendingEventsPerPlugin)
	outboundLimiter := outbound.NewMessageRateLimiter(state.core.Config)
	eventDispatcher.SetOutboundLimiter(outboundLimiter)
	eventBridge := bridge.New(state.core.Logger, eventDispatcher)
	eventBridge.SetAdapterStatsSource(adapterShell)
	eventBridge.SetDispatcherStatsSource(dispatcherStatsAdapter{dispatcher: eventDispatcher})
	eventDispatcher.SetRuntimePublisher(dispatcherRuntimePublisher{bridge: eventBridge})
	eventDispatcher.StartObservabilityFlush(dispatcherRuntimeFlushInterval)

	pluginRepository, pluginKVRepository, pluginConfigRepository, err := buildPluginRepositories(platform)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}
	webhookRegistry := pluginwebhook.NewRegistry()
	pluginFileService := pluginfile.NewService(filepath.Join(filepath.Dir(platform.Storage.Path), "plugins"))
	renderService, err := buildRenderService(state, platform, renderRunner)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}
	blacklistRepo := permission.NewSQLiteBlacklistRepository(platform.Storage.Read, platform.Storage.Write)
	whitelistRepo := permission.NewSQLiteWhitelistRepository(platform.Storage.Read, platform.Storage.Write)
	whitelistStateRepo := permission.NewSQLiteWhitelistStateRepository(platform.Storage.Read, platform.Storage.Write)

	if err := hydratePluginCatalog(state, pluginRepository, pluginConfigRepository); err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}
	cleanupOrphanedInstallDirs(state.core.Logger, state.discoverySpec.roots)
	if err := syncCatalogRenderTemplates(context.Background(), renderService, state.pluginCatalog); err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}

	pluginInstallService, pluginUninstallService, err := buildPluginMutationServices(state, pluginRepository)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}

	return appPlugins{
		Plugins:           state.pluginCatalog,
		Adapter:           adapterShell,
		Bridge:            eventBridge,
		Dispatcher:        eventDispatcher,
		replyTargets:      replyTargets,
		outboundSender:    adapterShell,
		PluginInstaller:   pluginInstallService,
		PluginUninstaller: pluginUninstallService,
		pluginRepository:  pluginRepository,
		pluginConfig:      pluginConfigRepository,
		pluginFiles:       pluginFileService,
		pluginKV:          pluginKVRepository,
		grantRepository:   pluginRepository,
		blacklistRepo:     blacklistRepo,
		whitelistRepo:     whitelistRepo,
		whitelistState:    whitelistStateRepo,
		webhooks:          webhookRegistry,
		renderer:          renderService,
		pluginLogLimiter:  localaction.NewPluginLogLimiter(state.core.Config),
		outboundLimiter:   outboundLimiter,
	}, nil
}

func buildPluginRepositories(platform appPlatform) (*plugins.SQLiteRepository, pluginkv.Repository, pluginconfig.Repository, error) {
	pluginRepository, err := plugins.NewSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin repository: %w", err)
	}
	pluginKVRepository, err := pluginkv.NewSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin kv repository: %w", err)
	}
	pluginConfigRepository, err := pluginconfig.NewSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin config repository: %w", err)
	}
	return pluginRepository, pluginKVRepository, pluginConfigRepository, nil
}

func buildRenderService(state appBuildState, platform appPlatform, renderRunner render.Runner) (*render.Service, error) {
	renderBrowserPath := prepareRenderBrowserPath(context.Background(), state.core.Logger, state.discoverySpec.repoRoot, state.core.Config.Render.BrowserPath)
	renderService, err := render.NewService(render.Options{
		RepoRoot:           state.discoverySpec.repoRoot,
		OutputRoot:         filepath.Join(filepath.Dir(platform.Storage.Path), "render"),
		Store:              platform.Storage,
		Runner:             renderRunner,
		WorkerCount:        state.core.Config.Render.WorkerCount,
		BrowserArgs:        state.core.Config.Render.BrowserArgs,
		BrowserPath:        renderBrowserPath,
		QueueMaxLength:     state.core.Config.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(state.core.Config.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(state.core.Config.Render.TimeoutSeconds) * time.Second,
		MaxRenderDataBytes: int(httpapi.MaxManagementJSONBodyBytes),
		FooterTemplate:     state.core.Config.Render.FooterTemplate,
		DefaultOutput:      state.core.Config.Render.DefaultOutput,
		DeviceScalePercent: state.core.Config.Render.DeviceScalePercent,
		Logger:             state.core.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create render service: %w", err)
	}
	return renderService, nil
}

func hydratePluginCatalog(state appBuildState, pluginRepository *plugins.SQLiteRepository, pluginConfigRepository pluginconfig.Repository) error {
	desiredStates, err := pluginRepository.LoadDesiredStates(context.Background())
	if err != nil {
		return fmt.Errorf("load persisted plugin desired_state: %w", err)
	}
	if packageLoader, ok := any(pluginRepository).(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			return fmt.Errorf("load plugin package metadata: %w", err)
		}
		state.pluginCatalog.Replace(plugins.ApplyPackageMetadata(state.pluginCatalog.List(), packageMetadata))
	}
	state.pluginCatalog.ApplyDesiredStates(desiredStates)
	if err := refreshCatalogCommandsFromSettings(context.Background(), state.pluginCatalog, pluginConfigRepository); err != nil {
		return err
	}
	return nil
}

func refreshCatalogCommandsFromSettings(ctx context.Context, catalog *plugins.Catalog, repo pluginconfig.Repository) error {
	if catalog == nil || repo == nil {
		return nil
	}
	for _, snapshot := range catalog.List() {
		settings := plugins.CloneSettings(snapshot.DefaultConfig)
		persisted, err := repo.ReadAll(ctx, snapshot.PluginID)
		if err != nil {
			return fmt.Errorf("load persisted plugin settings for %s: %w", snapshot.PluginID, err)
		}
		for key, value := range persisted {
			settings[key] = plugins.CloneSettingValue(value)
		}
		catalog.RefreshCommands(snapshot.PluginID, settings)
	}
	return nil
}

func buildPluginMutationServices(state appBuildState, pluginRepository *plugins.SQLiteRepository) (plugins.InstallCoordinator, plugins.UninstallCoordinator, error) {
	pluginInstallService, err := plugins.NewInstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.repoRoot,
		state.discoverySpec.roots,
		time.Duration(state.core.Config.Runtime.DependencyInstallTimeoutSecs)*time.Second,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin install service: %w", err)
	}
	pluginUninstallService, err := plugins.NewUninstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.repoRoot,
		state.discoverySpec.roots,
		nil,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin uninstall service: %w", err)
	}
	return pluginInstallService, pluginUninstallService, nil
}

func syncCatalogRenderTemplates(ctx context.Context, renderer *render.Service, catalog *plugins.Catalog) error {
	if renderer == nil || catalog == nil {
		return nil
	}
	return renderer.SyncPluginTemplates(ctx, pluginRenderTemplateSources(catalog.List()))
}

func pluginRenderTemplateSources(snapshots []plugins.Snapshot) []render.PluginTemplateSource {
	var sources []render.PluginTemplateSource
	seen := map[string]struct{}{}
	for _, snapshot := range snapshots {
		if !snapshot.Valid || snapshot.RegistrationState != "installed" {
			continue
		}
		for _, declared := range snapshot.RenderTemplates {
			dir, ok := pluginPackageRelativeDir(snapshot.PackageRootPath, declared.Path)
			if !ok {
				continue
			}
			key := snapshot.PluginID + "\x00" + dir
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sources = append(sources, render.PluginTemplateSource{
				PluginID:     snapshot.PluginID,
				Dir:          dir,
				ResourceRoot: snapshot.PackageRootPath,
			})
		}
	}
	return render.PluginTemplateSourcesFromManifests(sources)
}

func validatePluginRenderTemplates(snapshot plugins.Snapshot) error {
	var sources []render.PluginTemplateSource
	for _, declared := range snapshot.RenderTemplates {
		dir, ok := pluginPackageRelativeDir(snapshot.PackageRootPath, declared.Path)
		if !ok {
			return fmt.Errorf("plugin render template path %q is invalid", declared.Path)
		}
		sources = append(sources, render.PluginTemplateSource{
			PluginID:     snapshot.PluginID,
			Dir:          dir,
			ResourceRoot: snapshot.PackageRootPath,
		})
	}
	return render.ValidatePluginTemplateSources(sources)
}

func pluginPackageRelativeDir(packageRoot, relativePath string) (string, bool) {
	packageRoot = strings.TrimSpace(packageRoot)
	relativePath = strings.TrimSpace(relativePath)
	if packageRoot == "" || relativePath == "" || filepath.IsAbs(relativePath) {
		return "", false
	}
	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", false
	}
	absoluteRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(absoluteRoot, cleanRelative)
	relativeToRoot, err := filepath.Rel(absoluteRoot, candidate)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", false
	}
	return candidate, true
}

// dispatcherStatsAdapter projects the dispatcher's cumulative statistics into
// the smaller view the bridge observability frame consumes. It keeps the
// bridge-side interface free of an internal/dispatch import.
type dispatcherStatsAdapter struct {
	dispatcher *dispatch.Dispatcher
}

func (a dispatcherStatsAdapter) Stats() bridge.DispatcherStatsView {
	if a.dispatcher == nil {
		return bridge.DispatcherStatsView{}
	}
	stats := a.dispatcher.Stats()
	return bridge.DispatcherStatsView{
		Delivered: stats.Delivered,
		Dropped:   stats.Dropped,
		Errored:   stats.Errored,
		Ignored:   stats.Ignored,
	}
}

// dispatcherRuntimePublisher bridges dispatch window snapshots into a
// dispatcher_runtime observability frame on the bridge subscriber fan-out.
type dispatcherRuntimePublisher struct {
	bridge *bridge.Bridge
}

func (p dispatcherRuntimePublisher) PublishDispatcherRuntime(snap dispatch.DispatcherWindowSnapshot) {
	if p.bridge == nil {
		return
	}
	rows := make([]bridge.DispatcherRuntimeDropRow, 0, len(snap.DropsByReason))
	for _, row := range snap.DropsByReason {
		rows = append(rows, bridge.DispatcherRuntimeDropRow{
			Reason:    row.Reason,
			PluginID:  row.PluginID,
			EventType: row.EventType,
			Count:     row.Count,
		})
	}
	p.bridge.PublishDispatcherRuntime(bridge.DispatcherRuntimeData{
		WindowSeconds:  snap.WindowSeconds,
		DeliveredCount: snap.Delivered,
		DroppedCount:   snap.Dropped,
		IgnoredCount:   snap.Ignored,
		DropsByReason:  rows,
	})
}

// bridgeMetricsAdapter routes bridge outcomes into the platform-wide
// Prometheus registry. Inc helpers are no-ops when the registry is nil so
// tests can construct a Bridge without wiring a metrics observer.
type bridgeMetricsAdapter struct {
	registry *metrics.Registry
}

func (a bridgeMetricsAdapter) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}

func (a bridgeMetricsAdapter) IncBridgeIgnored() {
	if a.registry == nil || a.registry.BridgeIgnoredTotal == nil {
		return
	}
	a.registry.BridgeIgnoredTotal.Inc()
}

// dispatchMetricsAdapter routes dispatcher outcomes into the platform-wide
// Prometheus registry.
type dispatchMetricsAdapter struct {
	registry *metrics.Registry
}

func (a dispatchMetricsAdapter) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}

func (a dispatchMetricsAdapter) IncDispatcherDrop(pluginID, reason string) {
	if a.registry == nil || a.registry.DispatcherDropTotal == nil {
		return
	}
	a.registry.DispatcherDropTotal.WithLabelValues(pluginID, reason).Inc()
}

func (a dispatchMetricsAdapter) IncOutboundSend(adapterLabel, outcome string) {
	if a.registry == nil || a.registry.OutboundSendTotal == nil {
		return
	}
	a.registry.OutboundSendTotal.WithLabelValues(adapterLabel, outcome).Inc()
}

func (a dispatchMetricsAdapter) ObserveOutboundDuration(adapterLabel string, duration time.Duration) {
	if a.registry == nil || a.registry.OutboundSendDuration == nil {
		return
	}
	a.registry.OutboundSendDuration.WithLabelValues(adapterLabel).Observe(duration.Seconds())
}

// taskMetricsAdapter routes task executor outcomes into the platform-wide
// Prometheus registry.
type taskMetricsAdapter struct {
	registry *metrics.Registry
}

func (a taskMetricsAdapter) ObserveTaskExecution(taskType, outcome string, duration time.Duration) {
	if a.registry == nil || a.registry.TaskExecutionLatency == nil {
		return
	}
	a.registry.TaskExecutionLatency.WithLabelValues(taskType, outcome).Observe(duration.Seconds())
}

// renderMetricsAdapter routes render service outcomes into the platform-wide
// Prometheus registry. SetRenderQueueDepth is invoked from background
// goroutines so the gauge stays current without blocking the render loop.
type renderMetricsAdapter struct {
	registry *metrics.Registry
}

func (a renderMetricsAdapter) SetRenderQueueDepth(depth int) {
	if a.registry == nil || a.registry.RenderQueueDepth == nil {
		return
	}
	a.registry.RenderQueueDepth.Set(float64(depth))
}

func (a renderMetricsAdapter) ObserveRenderDuration(outcome string, duration time.Duration) {
	if a.registry == nil || a.registry.RenderDuration == nil {
		return
	}
	a.registry.RenderDuration.WithLabelValues(outcome).Observe(duration.Seconds())
}

// adapterMetricsAdapter routes adapter dedup observations into the
// platform-wide Prometheus registry.
type adapterMetricsAdapter struct {
	registry *metrics.Registry
}

func (a adapterMetricsAdapter) IncAdapterDedupDrop() {
	if a.registry == nil || a.registry.AdapterDedupDrops == nil {
		return
	}
	a.registry.AdapterDedupDrops.Inc()
}

func (a adapterMetricsAdapter) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}

// pluginRuntimeStates enumerates every formal plugin runtime state so the
// gauge resets stale buckets to zero on each refresh. New states must be
// added here to stay observable.
var pluginRuntimeStates = []string{
	"stopped",
	"starting",
	"running",
	"stopping",
	"crashed",
	"backoff",
	"dead_letter",
}

func refreshPluginRuntimeStateGauge(registry *metrics.Registry, catalog *plugins.Catalog) {
	if registry == nil || registry.PluginRuntimeState == nil || catalog == nil {
		return
	}
	counts := make(map[string]int, len(pluginRuntimeStates))
	for _, state := range pluginRuntimeStates {
		counts[state] = 0
	}
	for _, snapshot := range catalog.List() {
		state := strings.TrimSpace(snapshot.RuntimeState)
		if state == "" {
			continue
		}
		if _, ok := counts[state]; !ok {
			counts[state] = 0
		}
		counts[state]++
	}
	for state, count := range counts {
		registry.PluginRuntimeState.WithLabelValues(state).Set(float64(count))
	}
}

func startPluginRuntimeStateGaugeRefresh(registry *metrics.Registry, catalog *plugins.Catalog) (stop func()) {
	if registry == nil || catalog == nil {
		return func() {}
	}
	refreshPluginRuntimeStateGauge(registry, catalog)
	events, unsubscribe := catalog.Subscribe(16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case _, ok := <-events:
				if !ok {
					return
				}
				refreshPluginRuntimeStateGauge(registry, catalog)
			case <-ticker.C:
				refreshPluginRuntimeStateGauge(registry, catalog)
			}
		}
	}()
	return func() {
		unsubscribe()
		<-done
	}
}
