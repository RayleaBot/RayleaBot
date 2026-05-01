package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type schedulerTriggerProxy struct {
	mu      sync.RWMutex
	handler func(context.Context, scheduler.Job)
}

func newSchedulerTriggerProxy() *schedulerTriggerProxy {
	return &schedulerTriggerProxy{}
}

func (p *schedulerTriggerProxy) Set(handler func(context.Context, scheduler.Job)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = handler
}

func (p *schedulerTriggerProxy) Handle(ctx context.Context, job scheduler.Job) {
	if p == nil {
		return
	}
	p.mu.RLock()
	handler := p.handler
	p.mu.RUnlock()
	if handler != nil {
		handler(ctx, job)
	}
}

func (s *appRuntimeState) redactString(value string) string {
	if s == nil || s.redactText == nil {
		return value
	}
	return s.redactText(value)
}

func (s *appRuntimeState) recoverySummarySnapshot() *recovery.CompatibilitySummary {
	if s == nil {
		return nil
	}
	s.recoveryMu.RLock()
	defer s.recoveryMu.RUnlock()
	if s.recoverySummary == nil {
		return nil
	}
	summary := *s.recoverySummary
	if summary.UpdatedAt == "" {
		summary.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return &summary
}

func (s *appRuntimeState) setRecoverySummary(summary *recovery.CompatibilitySummary) {
	if s == nil {
		return
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	s.recoverySummary = summary
}

type pluginGrantView struct {
	state           *appRuntimeState
	plugins         *plugins.Catalog
	grantRepository plugins.GrantRepository
}

func (v *pluginGrantView) grantedCapabilities(ctx context.Context, pluginID string) []string {
	effective := v.effectiveGrants(ctx, pluginID)
	items := make([]string, 0, len(effective))
	for _, grant := range effective {
		items = append(items, grant.Capability)
	}
	return items
}

func (v *pluginGrantView) capabilityGranted(ctx context.Context, pluginID, capability string) bool {
	for _, granted := range v.grantedCapabilities(ctx, pluginID) {
		if strings.TrimSpace(granted) == capability {
			return true
		}
	}
	return false
}

func (v *pluginGrantView) CapabilityGranted(ctx context.Context, pluginID, capability string) bool {
	return v.capabilityGranted(ctx, pluginID, capability)
}

func (v *pluginGrantView) grantedScope(ctx context.Context, pluginID, capability string) grantedScope {
	for _, grant := range v.effectiveGrants(ctx, pluginID) {
		if strings.TrimSpace(grant.Capability) != capability {
			continue
		}
		scope := parseGrantedScope(grant.ScopeJSON)
		if len(scope.HTTPHosts) > 0 || len(scope.StorageRoots) > 0 || len(scope.Webhooks) > 0 {
			return scope
		}
	}

	return grantedScope{}
}

func (v *pluginGrantView) effectiveGrants(ctx context.Context, pluginID string) []plugins.EffectiveGrant {
	if v == nil {
		return nil
	}

	snapshot := plugins.Snapshot{PluginID: pluginID}
	if v.plugins != nil {
		if current, ok := v.plugins.Get(pluginID); ok {
			snapshot = current
		}
	}

	var persisted []plugins.PluginGrant
	if v.grantRepository != nil {
		grants, err := v.grantRepository.LoadGrants(ctx, pluginID)
		if err == nil {
			persisted = grants
		}
	}

	return plugins.ComputeEffectiveGrants(snapshot, currentAutoGrantCapabilities(v), persisted)
}

func currentAutoGrantCapabilities(v *pluginGrantView) []string {
	if v == nil || v.state == nil {
		return nil
	}
	if len(v.state.Config.Permission.AutoGrantCapabilities) > 0 {
		return append([]string(nil), v.state.Config.Permission.AutoGrantCapabilities...)
	}
	return append([]string(nil), v.state.Config.Auth.AutoGrantCapabilities...)
}

func (v *pluginGrantView) storageRootGranted(ctx context.Context, pluginID, root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	for _, grantedRoot := range v.grantedScope(ctx, pluginID, "storage.file").StorageRoots {
		if strings.TrimSpace(grantedRoot) == root {
			return true
		}
	}
	return false
}

func (v *pluginGrantView) StorageRootGranted(ctx context.Context, pluginID, root string) bool {
	return v.storageRootGranted(ctx, pluginID, root)
}

func (v *pluginGrantView) grantedWebhookScope(ctx context.Context, pluginID, route string) (plugins.WebhookScope, bool) {
	scope := v.grantedScope(ctx, pluginID, "event.expose_webhook")
	route = strings.TrimSpace(route)
	for _, item := range scope.Webhooks {
		if strings.TrimSpace(item.Route) == route {
			return item, true
		}
	}
	return plugins.WebhookScope{}, false
}

func (v *pluginGrantView) GrantedWebhookScope(ctx context.Context, pluginID, route string) (plugins.WebhookScope, bool) {
	return v.grantedWebhookScope(ctx, pluginID, route)
}

func (v *pluginGrantView) GrantedHTTPHosts(ctx context.Context, pluginID string) []string {
	return append([]string(nil), v.grantedScope(ctx, pluginID, "http.request").HTTPHosts...)
}

func (v *pluginGrantView) ListPluginSnapshots() []plugins.Snapshot {
	if v == nil || v.plugins == nil {
		return nil
	}
	return v.plugins.List()
}

type pluginLifecycleDeps struct {
	state            *appRuntimeState
	plugins          *plugins.Catalog
	desiredStateRepo plugins.DesiredStateRepository
	grants           *pluginGrantView
	runtimes         *runtimeRegistry
	dispatcher       *dispatch.Dispatcher
	pluginConfig     pluginconfig.Repository
	adapter          *adapter.Shell
	webhooks         *pluginwebhook.Registry
	onRecoveryChange func(string)
}

type eventMetadataEnricher interface {
	EnrichEventMetadata(context.Context, adapter.NormalizedEvent) adapter.NormalizedEvent
}

type eventIngressDeps struct {
	state            *appRuntimeState
	plugins          *plugins.Catalog
	replyTargets     *replyTargetCache
	outboundSender   outboundActionSender
	outboundLimiter  outbound.MessageLimiter
	bridge           *bridge.Bridge
	lifecycle        *pluginLifecycleController
	metadataEnricher eventMetadataEnricher
	whitelistRepo    permission.WhitelistRepository
	whitelistState   permission.WhitelistStateRepository
	blacklistRepo    permission.BlacklistRepository
}

type systemServiceDeps struct {
	state            *appRuntimeState
	auth             *auth.Manager
	adapter          *adapter.Shell
	plugins          *plugins.Catalog
	runtimes         *runtimeRegistry
	renderer         *render.Service
	pluginRepository plugins.DesiredStateRepository
	taskExecutor     *tasks.Executor
	logRepository    logging.Repository
}

type authHTTPDeps struct {
	state          *appRuntimeState
	auth           *auth.Manager
	loginFailures  *loginFailureTracker
	launcherTokens *launcherTokenStore
}

type managementHTTPDeps struct {
	state           *appRuntimeState
	auth            *auth.Manager
	launcherTokens  *launcherTokenStore
	system          *systemService
	requestShutdown func()
}

type configHTTPDeps struct {
	state            *appRuntimeState
	logs             *logging.Stream
	logRepository    logging.Repository
	renderer         *render.Service
	pluginLogLimiter *localaction.PluginLogLimiter
	outboundLimiter  interface{ ApplyConfig(config.Config) }
	protocol         *protocolService
	eventIngress     *eventIngressService
	blacklistRepo    permission.BlacklistRepository
}

type httpServerDeps struct {
	state              *appRuntimeState
	auth               *auth.Manager
	tasks              *tasks.Registry
	plugins            *plugins.Catalog
	logs               *logService
	console            *console.Stream
	pluginInstaller    plugins.InstallCoordinator
	pluginUninstaller  plugins.UninstallCoordinator
	pluginRepository   plugins.DesiredStateRepository
	grantRepository    plugins.GrantRepository
	pluginLifecycle    *pluginLifecycleController
	taskExecutor       *tasks.Executor
	renderer           *render.Service
	launcherTokens     *launcherTokenStore
	loginFailures      *loginFailureTracker
	configHandler      *configHTTPHandlers
	authHandler        *authHTTPHandlers
	managementHandler  *managementHTTPHandlers
	governanceHandler  *governance.Handlers
	taskHandler        *taskHTTPHandlers
	logHandler         *logHTTPHandlers
	renderHandler      *renderHTTPHandlers
	systemHandler      *systemHTTPHandlers
	protocolHandler    *protocolHTTPHandlers
	eventsWS           *eventsWSHandler
	tasksWS            *tasksWSHandler
	logsWS             *logsWSHandler
	consoleWS          *consoleWSHandler
	pluginWebhooks     *pluginwebhook.Service
	pluginManagementUI *pluginui.Handlers
}

type authHTTPHandlers struct {
	state          *appRuntimeState
	auth           *auth.Manager
	loginFailures  *loginFailureTracker
	launcherTokens *launcherTokenStore
}

func newAuthHTTPHandlers(deps authHTTPDeps) *authHTTPHandlers {
	return &authHTTPHandlers{
		state:          deps.state,
		auth:           deps.auth,
		loginFailures:  deps.loginFailures,
		launcherTokens: deps.launcherTokens,
	}
}

type managementHTTPHandlers struct {
	state           *appRuntimeState
	auth            *auth.Manager
	launcherTokens  *launcherTokenStore
	system          *systemService
	requestShutdown func()
}

func newManagementHTTPHandlers(deps managementHTTPDeps) *managementHTTPHandlers {
	return &managementHTTPHandlers{
		state:           deps.state,
		auth:            deps.auth,
		launcherTokens:  deps.launcherTokens,
		system:          deps.system,
		requestShutdown: deps.requestShutdown,
	}
}

type configHTTPHandlers struct {
	state            *appRuntimeState
	logs             *logging.Stream
	logRepository    logging.Repository
	renderer         *render.Service
	pluginLogLimiter *localaction.PluginLogLimiter
	outboundLimiter  interface{ ApplyConfig(config.Config) }
	protocol         *protocolService
	eventIngress     *eventIngressService
	blacklistRepo    permission.BlacklistRepository
}

func newConfigHTTPHandlers(deps configHTTPDeps) *configHTTPHandlers {
	return &configHTTPHandlers{
		state:            deps.state,
		logs:             deps.logs,
		logRepository:    deps.logRepository,
		renderer:         deps.renderer,
		pluginLogLimiter: deps.pluginLogLimiter,
		outboundLimiter:  deps.outboundLimiter,
		protocol:         deps.protocol,
		eventIngress:     deps.eventIngress,
		blacklistRepo:    deps.blacklistRepo,
	}
}

type logService struct {
	stream     *logging.Stream
	repository logging.Repository
}

func newLogService(stream *logging.Stream, repository logging.Repository) *logService {
	return &logService{stream: stream, repository: repository}
}

func (s *logService) currentBootID() string {
	if s == nil || s.stream == nil {
		return ""
	}
	return s.stream.BootID()
}

type logHTTPHandlers struct {
	logs *logService
}

func newLogHTTPHandlers(logs *logService) *logHTTPHandlers {
	return &logHTTPHandlers{logs: logs}
}

type taskHTTPHandlers struct {
	tasks           *tasks.Registry
	taskExecutor    *tasks.Executor
	pluginInstaller plugins.InstallCoordinator
}

func newTaskHTTPHandlers(taskRegistry *tasks.Registry, taskExecutor *tasks.Executor, pluginInstaller plugins.InstallCoordinator) *taskHTTPHandlers {
	return &taskHTTPHandlers{
		tasks:           taskRegistry,
		taskExecutor:    taskExecutor,
		pluginInstaller: pluginInstaller,
	}
}

type renderHTTPHandlers struct {
	renderer     *render.Service
	taskExecutor *tasks.Executor
}

func newRenderHTTPHandlers(renderer *render.Service, taskExecutor *tasks.Executor) *renderHTTPHandlers {
	return &renderHTTPHandlers{
		renderer:     renderer,
		taskExecutor: taskExecutor,
	}
}

type systemHTTPHandlers struct {
	system *systemService
}

func newSystemHTTPHandlers(system *systemService) *systemHTTPHandlers {
	return &systemHTTPHandlers{system: system}
}

type protocolHTTPHandlers struct {
	protocol *protocolService
}

func newProtocolHTTPHandlers(protocol *protocolService) *protocolHTTPHandlers {
	return &protocolHTTPHandlers{protocol: protocol}
}

type eventsWSHandler struct {
	bridge        *bridge.Bridge
	plugins       *plugins.Catalog
	protocol      *protocolService
	serviceStatus *serviceStatusService
	governance    *governanceEventService
}

func newEventsWSHandler(bridge *bridge.Bridge, plugins *plugins.Catalog, protocol *protocolService, serviceStatus *serviceStatusService, governance *governanceEventService) *eventsWSHandler {
	return &eventsWSHandler{bridge: bridge, plugins: plugins, protocol: protocol, serviceStatus: serviceStatus, governance: governance}
}

type tasksWSHandler struct {
	tasks *tasks.Registry
}

func newTasksWSHandler(tasks *tasks.Registry) *tasksWSHandler {
	return &tasksWSHandler{tasks: tasks}
}

type logsWSHandler struct {
	logs *logService
}

func newLogsWSHandler(logs *logService) *logsWSHandler {
	return &logsWSHandler{logs: logs}
}

type consoleWSHandler struct {
	console *console.Stream
	plugins *plugins.Catalog
}

func newConsoleWSHandler(console *console.Stream, plugins *plugins.Catalog) *consoleWSHandler {
	return &consoleWSHandler{console: console, plugins: plugins}
}

type webhookGateway interface {
	Expose(context.Context, string, runtime.Action) (map[string]any, error)
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type readinessProvider interface {
	CurrentReadiness() health.ReadinessReport
}

var _ readinessProvider = (*systemService)(nil)
var _ http.Handler = (http.Handler)(nil)

type grantedScope struct {
	HTTPHosts    []string               `json:"http_hosts"`
	StorageRoots []string               `json:"storage_roots"`
	Webhooks     []plugins.WebhookScope `json:"webhooks"`
}

func parseGrantedScope(raw string) grantedScope {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return grantedScope{}
	}
	var scope grantedScope
	if err := json.Unmarshal([]byte(raw), &scope); err != nil {
		return grantedScope{}
	}
	return scope
}
