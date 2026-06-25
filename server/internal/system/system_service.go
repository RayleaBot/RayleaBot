package system

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type StatusPublisher interface {
	PublishSnapshot()
}

type AuthBootstrapState interface {
	IsBootstrapped() bool
}

type AdapterStateSource interface {
	CurrentState() string
}

type RuntimeRegistry interface {
	ActiveCount() int
}

type RendererState interface {
	Diagnostics() []health.DiagnosticIssue
	RefreshBrowserPath(string)
}

type DiagnosticsThirdParty = systemmodel.DiagnosticsThirdParty
type DiagnosticsThirdPartyPlatform = systemmodel.DiagnosticsThirdPartyPlatform
type DiagnosticsBilibiliSource = systemmodel.DiagnosticsBilibiliSource
type DiagnosticsScheduler = systemmodel.DiagnosticsScheduler

type ThirdPartyDiagnosticsSource interface {
	DiagnosticsThirdParty(context.Context) (DiagnosticsThirdParty, []health.DiagnosticIssue)
}

type BilibiliSourceDiagnosticsSource interface {
	DiagnosticsBilibiliSource(context.Context) (DiagnosticsBilibiliSource, []health.DiagnosticIssue)
}

type SchedulerDiagnosticsSource interface {
	DiagnosticsScheduler() DiagnosticsScheduler
}

type Deps struct {
	CurrentConfig    func() config.Config
	CurrentSummary   func() config.Summary
	CurrentRepoRoot  func() string
	CurrentStartedAt func() time.Time
	RepoRoot         string
	Logger           *slog.Logger
	StartedAt        time.Time
	Auth             AuthBootstrapState
	Adapter          AdapterStateSource
	Plugins          plugins.CatalogView
	Runtimes         RuntimeRegistry
	Renderer         RendererState
	Storage          *storage.Store
	ThirdParty       ThirdPartyDiagnosticsSource
	BilibiliSource   BilibiliSourceDiagnosticsSource
	Scheduler        SchedulerDiagnosticsSource
	PluginRepository plugins.DesiredStateRepository
	TaskExecutor     *tasks.Executor
	LogRepository    logging.Repository
}

type Service struct {
	currentConfig    func() config.Config
	currentSummary   func() config.Summary
	currentRepoRoot  func() string
	currentStartedAt func() time.Time
	repoRoot         string
	logger           *slog.Logger
	startedAt        time.Time
	auth             AuthBootstrapState
	adapter          AdapterStateSource
	plugins          plugins.CatalogView
	runtimes         RuntimeRegistry
	renderer         RendererState
	storage          *storage.Store
	thirdParty       ThirdPartyDiagnosticsSource
	bilibiliSource   BilibiliSourceDiagnosticsSource
	scheduler        SchedulerDiagnosticsSource
	pluginRepository plugins.DesiredStateRepository
	taskExecutor     *tasks.Executor
	logRepository    logging.Repository
	shuttingDown     *atomic.Bool
	statusPublisher  StatusPublisher
	recoveryMu       sync.RWMutex
	recoverySummary  *recovery.CompatibilitySummary
	startupMu        sync.RWMutex
	startupRuntimes  map[string]StartupRuntimeState
}

func New(deps Deps) *Service {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	currentConfig := deps.CurrentConfig
	if currentConfig == nil {
		currentConfig = func() config.Config { return config.Config{} }
	}
	currentSummary := deps.CurrentSummary
	if currentSummary == nil {
		currentSummary = func() config.Summary { return config.Summary{} }
	}
	authState := deps.Auth
	if isNilDependency(authState) {
		authState = nil
	}
	adapter := deps.Adapter
	if isNilDependency(adapter) {
		adapter = nil
	}
	pluginsCatalog := deps.Plugins
	if isNilDependency(pluginsCatalog) {
		pluginsCatalog = nil
	}
	runtimes := deps.Runtimes
	if isNilDependency(runtimes) {
		runtimes = nil
	}
	renderer := deps.Renderer
	if isNilDependency(renderer) {
		renderer = nil
	}
	return &Service{
		currentConfig:    currentConfig,
		currentSummary:   currentSummary,
		currentRepoRoot:  deps.CurrentRepoRoot,
		currentStartedAt: deps.CurrentStartedAt,
		repoRoot:         deps.RepoRoot,
		logger:           deps.Logger,
		startedAt:        deps.StartedAt,
		auth:             authState,
		adapter:          adapter,
		plugins:          pluginsCatalog,
		runtimes:         runtimes,
		renderer:         renderer,
		storage:          deps.Storage,
		thirdParty:       deps.ThirdParty,
		bilibiliSource:   deps.BilibiliSource,
		scheduler:        deps.Scheduler,
		pluginRepository: deps.PluginRepository,
		taskExecutor:     deps.TaskExecutor,
		logRepository:    deps.LogRepository,
		startupRuntimes:  newStartupRuntimeStates(nil),
	}
}

func (s *Service) SystemStatus() string {
	return s.systemStatus()
}

func (s *Service) SchedulerPluginName(pluginID string) string {
	pluginName := strings.TrimSpace(pluginID)
	if s != nil && s.plugins != nil {
		if snapshot, ok := s.plugins.Get(pluginID); ok {
			if name := strings.TrimSpace(snapshot.Name); name != "" {
				pluginName = name
			}
		}
	}
	if pluginName == "" {
		return "未知插件"
	}
	return pluginName
}

func (s *Service) SchedulerTimezone() string {
	if s != nil {
		if tz := strings.TrimSpace(s.config().Scheduler.Timezone); tz != "" {
			return tz
		}
	}
	return "UTC"
}

func (s *Service) StatusSnapshot() systemmodel.StatusSnapshot {
	adapterState := ""
	if s != nil && s.adapter != nil {
		adapterState = s.adapter.CurrentState()
	}
	runningPlugins, failedPlugins := s.pluginStateCounts()
	return systemmodel.StatusSnapshot{
		Status:          s.systemStatus(),
		AdapterState:    adapterState,
		ActivePlugins:   s.activePluginCount(),
		RunningPlugins:  runningPlugins,
		FailedPlugins:   failedPlugins,
		DBSchemaVersion: s.dbSchemaVersion(),
		UptimeSeconds:   s.uptimeSeconds(),
		RecoverySummary: s.recoverySummarySnapshot(),
		Health:          readinessReportPtr(s.CurrentReadiness()),
	}
}

func readinessReportPtr(report health.ReadinessReport) *health.ReadinessReport {
	return &report
}

func (s *Service) SetAuth(manager AuthBootstrapState) {
	if s != nil {
		if isNilDependency(manager) {
			manager = nil
		}
		s.auth = manager
	}
}

func (s *Service) SetLogRepository(repository logging.Repository) {
	if s != nil {
		s.logRepository = repository
	}
}

func (s *Service) SetStatusPublisher(publisher StatusPublisher) {
	if s != nil {
		s.statusPublisher = publisher
	}
}

func (s *Service) BindShutdownFlag(flag *atomic.Bool) {
	if s != nil {
		s.shuttingDown = flag
	}
}

func (s *Service) config() config.Config {
	if s == nil || s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func (s *Service) summary() config.Summary {
	if s == nil || s.currentSummary == nil {
		return config.Summary{}
	}
	return s.currentSummary()
}

func (s *Service) repoRootPath() string {
	if s == nil {
		return ""
	}
	if s.currentRepoRoot != nil {
		return s.currentRepoRoot()
	}
	return s.repoRoot
}

func (s *Service) startedAtValue() time.Time {
	if s == nil {
		return time.Time{}
	}
	if s.currentStartedAt != nil {
		return s.currentStartedAt()
	}
	return s.startedAt
}

func (s *Service) currentLogger() *slog.Logger {
	if s == nil || s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

func isNilDependency(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func (s *Service) recoverySummarySnapshot() *recovery.CompatibilitySummary {
	if s == nil {
		return nil
	}
	s.recoveryMu.RLock()
	defer s.recoveryMu.RUnlock()
	if s.recoverySummary == nil {
		return nil
	}
	copied := *s.recoverySummary
	copied.Issues = append([]recovery.CompatibilityIssue(nil), s.recoverySummary.Issues...)
	copied.ManualActions = append([]string(nil), s.recoverySummary.ManualActions...)
	copied.NextSteps = append([]string(nil), s.recoverySummary.NextSteps...)
	copied.SkippedPlugins = append([]recovery.SkippedPlugin(nil), s.recoverySummary.SkippedPlugins...)
	copied.Audit = append([]recovery.AuditEntry(nil), s.recoverySummary.Audit...)
	return &copied
}

func (s *Service) setRecoverySummary(summary *recovery.CompatibilitySummary) {
	if s == nil {
		return
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if summary == nil {
		s.recoverySummary = nil
		return
	}
	copied := *summary
	copied.Issues = append([]recovery.CompatibilityIssue(nil), summary.Issues...)
	copied.ManualActions = append([]string(nil), summary.ManualActions...)
	copied.NextSteps = append([]string(nil), summary.NextSteps...)
	copied.SkippedPlugins = append([]recovery.SkippedPlugin(nil), summary.SkippedPlugins...)
	copied.Audit = append([]recovery.AuditEntry(nil), summary.Audit...)
	s.recoverySummary = &copied
}

var _ interface {
	SystemStatus() string
	CurrentReadiness() health.ReadinessReport
} = (*Service)(nil)
