package system

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
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
	CurrentState() string
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
	Adapter             AdapterStateSource
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
	adapter             AdapterStateSource
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
	shuttingDown        *atomic.Bool
	statusPublisher     StatusPublisher
	recoveryMu          sync.RWMutex
	recoverySummary     *recovery.CompatibilitySummary
	startupMu           sync.RWMutex
	startupRuntimes     map[string]StartupRuntimeState
}

func New(deps Deps) (*Service, error) {
	if deps.CurrentConfig == nil || deps.CurrentSummary == nil || deps.Plugins == nil {
		return nil, fmt.Errorf("system service requires current config, summary and plugin catalog")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
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
		adapter:             deps.Adapter,
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

func (s *Service) SchedulerPluginName(pluginID string) string {
	pluginName := strings.TrimSpace(pluginID)
	if s.plugins != nil {
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
		if s.scheduler != nil {
			return s.scheduler.Timezone()
		}
		if tz := strings.TrimSpace(s.config().Scheduler.Timezone); tz != "" {
			return tz
		}
	}
	return config.DefaultTimezone
}

func (s *Service) StatusSnapshot() StatusSnapshot {
	adapterState := ""
	if s.adapter != nil {
		adapterState = s.adapter.CurrentState()
	}
	runningPlugins, failedPlugins := s.pluginStateCounts()
	return StatusSnapshot{
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
	copied.Issues = append([]recovery.CompatibilityIssue(nil), s.recoverySummary.Issues...)
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
