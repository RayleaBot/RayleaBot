package system

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/runtime/registry"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type StatusPublisher interface {
	PublishSnapshot()
}

type Deps struct {
	CurrentConfig    func() config.Config
	CurrentSummary   func() config.Summary
	CurrentRepoRoot  func() string
	CurrentStartedAt func() time.Time
	RepoRoot         string
	Logger           *slog.Logger
	StartedAt        time.Time
	Auth             *auth.Manager
	Adapter          *adaptershell.Shell
	Plugins          *plugincatalog.Catalog
	Runtimes         *runtimeregistry.Registry
	Renderer         *renderservice.Service
	Storage          *storage.Store
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
	auth             *auth.Manager
	adapter          *adaptershell.Shell
	plugins          *plugincatalog.Catalog
	runtimes         *runtimeregistry.Registry
	renderer         *renderservice.Service
	storage          *storage.Store
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
	return &Service{
		currentConfig:    currentConfig,
		currentSummary:   currentSummary,
		currentRepoRoot:  deps.CurrentRepoRoot,
		currentStartedAt: deps.CurrentStartedAt,
		repoRoot:         deps.RepoRoot,
		logger:           deps.Logger,
		startedAt:        deps.StartedAt,
		auth:             deps.Auth,
		adapter:          deps.Adapter,
		plugins:          deps.Plugins,
		runtimes:         deps.Runtimes,
		renderer:         deps.Renderer,
		storage:          deps.Storage,
		pluginRepository: deps.PluginRepository,
		taskExecutor:     deps.TaskExecutor,
		logRepository:    deps.LogRepository,
		startupRuntimes:  newStartupRuntimeStates(nil),
	}
}

func (s *Service) SystemStatus() string {
	return s.systemStatus()
}

func (s *Service) ManagementStatusSnapshot() managementhttp.SystemStatusResponse {
	adapterState := ""
	if s != nil && s.adapter != nil {
		adapterState = string(stateOrIdle(s.adapter.Snapshot().State))
	}
	return managementhttp.SystemStatusResponse{
		Status:          s.systemStatus(),
		AdapterState:    adapterState,
		ActivePlugins:   s.activePluginCount(),
		UptimeSeconds:   s.uptimeSeconds(),
		RecoverySummary: s.recoverySummarySnapshot(),
	}
}

func (s *Service) SetAuth(manager *auth.Manager) {
	if s != nil {
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

var _ managementevents.ServiceStatusProvider = (*Service)(nil)
