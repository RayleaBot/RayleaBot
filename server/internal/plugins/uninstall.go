package plugins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/tasks"
)

const (
	codePluginUninstallFailed = "plugin.uninstall_failed"
)

// StopPluginFunc stops the runtime for the given plugin if it is running.
// It is injected by the app layer to avoid an import cycle with the runtime package.
type StopPluginFunc func(pluginID string)

type UninstallService struct {
	logger         *slog.Logger
	registry       *tasks.Registry
	catalog        *Catalog
	repository     DesiredStateRepository
	packageRepo    PackageRepository
	validator      *schema.Validator
	repoRoot       string
	discoveryRoots []ScanRoot
	installedRoot  string
	stopPlugin     StopPluginFunc

	baseCtx    context.Context
	baseCancel context.CancelFunc
	wg         sync.WaitGroup
	jobs       chan uninstallJob

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	deps    uninstallerDeps
}

type uninstallerDeps struct {
	now       func() time.Time
	removeAll func(string) error
	stat      func(string) (os.FileInfo, error)
}

type uninstallJob struct {
	taskID   string
	pluginID string
	ctx      context.Context
}

func NewUninstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog *Catalog,
	repository DesiredStateRepository,
	validator *schema.Validator,
	repoRoot string,
	discoveryRoots []ScanRoot,
	stopPlugin StopPluginFunc,
) (*UninstallService, error) {
	if registry == nil {
		return nil, errors.New("task registry is required")
	}
	if catalog == nil {
		return nil, errors.New("plugin catalog is required")
	}

	installedRoot := ""
	for _, root := range discoveryRoots {
		if root.Label == "plugins/installed" {
			installedRoot = root.Path
			break
		}
	}
	if installedRoot == "" {
		return nil, errors.New("plugins/installed discovery root is required")
	}

	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	var packageRepo PackageRepository
	if repo, ok := repository.(PackageRepository); ok {
		packageRepo = repo
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())
	service := &UninstallService{
		logger:         logger,
		registry:       registry,
		catalog:        catalog,
		repository:     repository,
		packageRepo:    packageRepo,
		validator:      validator,
		repoRoot:       repoRoot,
		discoveryRoots: append([]ScanRoot(nil), discoveryRoots...),
		installedRoot:  installedRoot,
		stopPlugin:     stopPlugin,
		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		jobs:           make(chan uninstallJob, 32),
		cancels:        map[string]context.CancelFunc{},
		deps: uninstallerDeps{
			now:       time.Now,
			removeAll: os.RemoveAll,
			stat:      os.Stat,
		},
	}

	service.wg.Add(1)
	go service.run()
	return service, nil
}

func (s *UninstallService) SetStopPlugin(fn StopPluginFunc) {
	s.stopPlugin = fn
}

func (s *UninstallService) Accept(_ context.Context, pluginID string) (string, error) {
	summary := fmt.Sprintf("uninstall plugin: %s", pluginID)
	taskID, err := s.registry.Create("plugin.uninstall", summary)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(s.baseCtx, 5*time.Minute)
	s.mu.Lock()
	s.cancels[taskID] = cancel
	s.mu.Unlock()

	select {
	case s.jobs <- uninstallJob{taskID: taskID, pluginID: pluginID, ctx: runCtx}:
		return taskID, nil
	case <-s.baseCtx.Done():
		cancel()
		return "", errors.New("uninstall service is shutting down")
	}
}

func (s *UninstallService) Close() error {
	if s == nil {
		return nil
	}
	s.baseCancel()

	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.cancels))
	for _, cancel := range s.cancels {
		cancels = append(cancels, cancel)
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}

	s.wg.Wait()
	return nil
}

func (s *UninstallService) run() {
	defer s.wg.Done()
	for {
		select {
		case <-s.baseCtx.Done():
			return
		case job := <-s.jobs:
			s.execute(job)
		}
	}
}

func (s *UninstallService) execute(job uninstallJob) {
	defer s.dropCancel(job.taskID)

	startedAt := s.deps.now().UTC()
	s.registry.Update(job.taskID, tasks.Update{
		Status:    taskStatusPtr(tasks.StatusRunning),
		Progress:  intPtr(10),
		Summary:   stringPtr("停止插件运行时"),
		StartedAt: &startedAt,
	})

	// Stop runtime if this plugin is currently running.
	if s.stopPlugin != nil {
		s.stopPlugin(job.pluginID)
	}

	if err := job.ctx.Err(); err != nil {
		s.failTask(job.taskID, codePluginUninstallFailed, "插件卸载已取消", "插件卸载已取消")
		return
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(30),
		Summary:  stringPtr("清理数据库记录"),
	})

	// Remove desired_state record.
	if s.repository != nil {
		if err := s.repository.DeleteDesiredState(job.ctx, job.pluginID); err != nil {
			s.logger.Warn("delete desired_state during uninstall", "plugin_id", job.pluginID, "err", err.Error())
		}
	}

	// Remove package metadata.
	if s.packageRepo != nil {
		if err := s.packageRepo.DeletePackageMetadata(job.ctx, job.pluginID); err != nil {
			s.logger.Warn("delete package metadata during uninstall", "plugin_id", job.pluginID, "err", err.Error())
		}
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(50),
		Summary:  stringPtr("删除插件安装目录"),
	})

	// Remove plugin directory.
	pluginDir := filepath.Join(s.installedRoot, job.pluginID)
	if _, err := s.deps.stat(pluginDir); err == nil {
		if err := s.deps.removeAll(pluginDir); err != nil {
			s.failTask(job.taskID, codePluginUninstallFailed, "删除插件安装目录失败", "删除插件安装目录失败")
			return
		}
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(80),
		Summary:  stringPtr("刷新插件目录索引"),
	})

	// Refresh catalog.
	if err := s.refreshCatalog(); err != nil {
		s.failTask(job.taskID, codePluginUninstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败")
		return
	}

	now := s.deps.now().UTC()
	s.registry.Update(job.taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusSucceeded),
		Progress:   intPtr(100),
		Summary:    stringPtr("插件卸载完成"),
		FinishedAt: &now,
		Result: &tasks.ResultSummary{
			Summary: "插件已卸载并刷新插件目录索引",
		},
	})
}

func (s *UninstallService) refreshCatalog() error {
	snapshots, _, err := Discover(DiscoverOptions{
		Validator: s.validator,
		Roots:     s.discoveryRoots,
		RepoRoot:  s.repoRoot,
		Logger:    s.logger,
	})
	if err != nil {
		return err
	}

	reloaded := NewCatalog(snapshots)
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(context.Background())
		if err != nil {
			return err
		}
		reloaded.ApplyDesiredStates(states)
	}

	s.catalog.Replace(reloaded.List())
	return nil
}

func (s *UninstallService) failTask(taskID, code, message, summary string) {
	now := s.deps.now().UTC()
	s.registry.Update(taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusFailed),
		Summary:    stringPtr(summary),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
		},
	})
}

func (s *UninstallService) dropCancel(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cancels, taskID)
}
