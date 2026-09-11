package system

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/operations/recovery"
	runtimedeps "github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type StatusPublisher interface {
	PublishSnapshot()
}

type AuthBootstrapState interface {
	IsBootstrapped() bool
}

type AdapterStateSource interface {
	AdapterStates() []adapterservice.Status
}

type RuntimeRegistry interface {
	ActiveCount() int
}

type RendererState interface {
	Diagnostics() []health.DiagnosticIssue
	RefreshBrowserPath(string)
}

type DatabasePathResolver func(configPath, databasePath string) (string, error)

type ThirdPartyDiagnosticsSource interface {
	DiagnosticsThirdParty(context.Context) (DiagnosticsThirdParty, []health.DiagnosticIssue)
}

type SchedulerDiagnosticsSource interface {
	DiagnosticsScheduler() DiagnosticsScheduler
	Timezone() string
}

type Deps struct {
	CurrentConfig       func() config.Config
	CurrentSummary      func() config.Summary
	CurrentRepoRoot     func() string
	CurrentStartedAt    func() time.Time
	RepoRoot            string
	Logger              *slog.Logger
	StartedAt           time.Time
	Auth                AuthBootstrapState
	Adapters            AdapterStateSource
	Plugins             plugins.CatalogView
	Runtimes            RuntimeRegistry
	Renderer            RendererState
	Storage             *storage.Store
	ThirdParty          ThirdPartyDiagnosticsSource
	Scheduler           SchedulerDiagnosticsSource
	PluginRepository    plugins.DesiredStateRepository
	TaskExecutor        *tasks.Executor
	LogRepository       logging.Repository
	StatusPublisher     StatusPublisher
	ResolveDatabasePath DatabasePathResolver
	InspectRuntime      func(string, string) (*runtimedeps.BootstrapInspection, error)
	PrepareRuntime      func(context.Context, string, string, runtimedeps.PrepareProgressReporter) (*runtimedeps.PrepareReport, error)
}

type Service struct {
	currentConfig       func() config.Config
	currentSummary      func() config.Summary
	currentRepoRoot     func() string
	currentStartedAt    func() time.Time
	repoRoot            string
	logger              *slog.Logger
	startedAt           time.Time
	auth                AuthBootstrapState
	adapters            AdapterStateSource
	plugins             plugins.CatalogView
	runtimes            RuntimeRegistry
	renderer            RendererState
	storage             *storage.Store
	thirdParty          ThirdPartyDiagnosticsSource
	scheduler           SchedulerDiagnosticsSource
	pluginRepository    plugins.DesiredStateRepository
	taskExecutor        *tasks.Executor
	logRepository       logging.Repository
	resolveDatabasePath DatabasePathResolver
	inspectRuntime      func(string, string) (*runtimedeps.BootstrapInspection, error)
	prepareRuntime      func(context.Context, string, string, runtimedeps.PrepareProgressReporter) (*runtimedeps.PrepareReport, error)
	shuttingDown        *atomic.Bool
	statusPublisher     StatusPublisher
	recoveryMu          sync.RWMutex
	recoverySummary     *recovery.CompatibilitySummary
	startupMu           sync.RWMutex
	startupRuntimes     map[string]StartupRuntimeState
}

func New(deps Deps) (*Service, error) {
	if deps.CurrentConfig == nil || deps.CurrentSummary == nil || deps.Plugins == nil || deps.PluginRepository == nil {
		return nil, fmt.Errorf("system service requires current config, summary, plugin catalog and desired state repository")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.InspectRuntime == nil {
		deps.InspectRuntime = inspectRuntime
	}
	if deps.PrepareRuntime == nil {
		deps.PrepareRuntime = prepareRuntime
	}
	return &Service{
		currentConfig:       deps.CurrentConfig,
		currentSummary:      deps.CurrentSummary,
		currentRepoRoot:     deps.CurrentRepoRoot,
		currentStartedAt:    deps.CurrentStartedAt,
		repoRoot:            deps.RepoRoot,
		logger:              deps.Logger,
		startedAt:           deps.StartedAt,
		auth:                deps.Auth,
		adapters:            deps.Adapters,
		plugins:             deps.Plugins,
		runtimes:            deps.Runtimes,
		renderer:            deps.Renderer,
		storage:             deps.Storage,
		thirdParty:          deps.ThirdParty,
		scheduler:           deps.Scheduler,
		pluginRepository:    deps.PluginRepository,
		taskExecutor:        deps.TaskExecutor,
		logRepository:       deps.LogRepository,
		statusPublisher:     deps.StatusPublisher,
		resolveDatabasePath: databasePathResolver(deps.ResolveDatabasePath),
		inspectRuntime:      deps.InspectRuntime,
		prepareRuntime:      deps.PrepareRuntime,
		startupRuntimes:     newStartupRuntimeStates(nil),
	}, nil
}

func databasePathResolver(resolver DatabasePathResolver) DatabasePathResolver {
	if resolver != nil {
		return resolver
	}
	return runtimepaths.ResolveDatabasePath
}

func (s *Service) databasePath(configPath, configuredPath string) (string, error) {
	return s.resolveDatabasePath(configPath, configuredPath)
}

func (s *Service) SystemStatus() string {
	return s.systemStatus()
}

func (s *Service) StatusSnapshot() StatusSnapshot {
	adapters := []adapterservice.Status{}
	if s.adapters != nil {
		adapters = append(adapters, s.adapters.AdapterStates()...)
	}
	runningPlugins, failedPlugins := s.pluginStateCounts()
	return StatusSnapshot{
		Status:          s.systemStatus(),
		Adapters:        adapters,
		ActivePlugins:   s.activePluginCount(),
		RunningPlugins:  runningPlugins,
		FailedPlugins:   failedPlugins,
		DBSchemaVersion: s.dbSchemaVersion(),
		UptimeSeconds:   s.uptimeSeconds(),
		RecoverySummary: s.recoverySummarySnapshot(),
		Health:          readinessReportPtr(s.CurrentReadiness()),
	}
}

func readinessReportPtr(report ReadinessReport) *ReadinessReport {
	return &report
}

func (s *Service) BindShutdownFlag(flag *atomic.Bool) {
	if s != nil {
		s.shuttingDown = flag
	}
}

func (s *Service) config() config.Config {
	return s.currentConfig()
}

func (s *Service) summary() config.Summary {
	return s.currentSummary()
}

func (s *Service) repoRootPath() string {
	if s.currentRepoRoot != nil {
		return s.currentRepoRoot()
	}
	return s.repoRoot
}

func (s *Service) startedAtValue() time.Time {
	if s.currentStartedAt != nil {
		return s.currentStartedAt()
	}
	return s.startedAt
}

func (s *Service) currentLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

func (s *Service) recoverySummarySnapshot() *recovery.CompatibilitySummary {
	s.recoveryMu.RLock()
	defer s.recoveryMu.RUnlock()
	if s.recoverySummary == nil {
		return nil
	}
	copied := *s.recoverySummary
	copied.Issues = copyRecoveryIssues(s.recoverySummary.Issues)
	copied.ManualActions = append([]string(nil), s.recoverySummary.ManualActions...)
	copied.NextSteps = append([]string(nil), s.recoverySummary.NextSteps...)
	copied.SkippedPlugins = append([]recovery.SkippedPlugin(nil), s.recoverySummary.SkippedPlugins...)
	copied.Audit = append([]recovery.AuditEntry(nil), s.recoverySummary.Audit...)
	return &copied
}

func (s *Service) setRecoverySummary(summary *recovery.CompatibilitySummary) {
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if summary == nil {
		s.recoverySummary = nil
		return
	}
	copied := *summary
	copied.Issues = copyRecoveryIssues(summary.Issues)
	copied.ManualActions = append([]string(nil), summary.ManualActions...)
	copied.NextSteps = append([]string(nil), summary.NextSteps...)
	copied.SkippedPlugins = append([]recovery.SkippedPlugin(nil), summary.SkippedPlugins...)
	copied.Audit = append([]recovery.AuditEntry(nil), summary.Audit...)
	s.recoverySummary = &copied
}

var _ interface {
	SystemStatus() string
	CurrentReadiness() ReadinessReport
} = (*Service)(nil)

func copyRecoveryIssues(issues []recovery.CompatibilityIssue) []recovery.CompatibilityIssue {
	copied := append([]recovery.CompatibilityIssue(nil), issues...)
	for i := range copied {
		copied[i].RuntimeResources = append([]string(nil), issues[i].RuntimeResources...)
	}
	return copied
}
