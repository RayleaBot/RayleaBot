package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codePluginUninstallFailed = "plugin.uninstall_failed"
)

type UninstallService struct {
	logger         *slog.Logger
	registry       *tasks.Registry
	catalog        plugins.CatalogStore
	repository     plugins.DesiredStateRepository
	packageRepo    plugins.PackageRepository
	validator      *config.Validator
	repoRoot       string
	discoveryRoots []plugincatalog.ScanRoot
	installedRoot  string
	stopPlugin     plugins.StopPluginFunc

	baseCtx    context.Context
	baseCancel context.CancelFunc
	wg         sync.WaitGroup
	jobs       chan uninstallJob
	admission  *tasks.QueueAdmission

	mu      sync.Mutex
	closed  bool
	cancels map[string]context.CancelFunc
	deps    uninstallerDeps

	afterSuccess func(context.Context, string)
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
	catalog plugins.CatalogStore,
	repository plugins.DesiredStateRepository,
	validator *config.Validator,
	repoRoot string,
	discoveryRoots []plugincatalog.ScanRoot,
	stopPlugin plugins.StopPluginFunc,
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

	var packageRepo plugins.PackageRepository
	if repo, ok := repository.(plugins.PackageRepository); ok {
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
		discoveryRoots: append([]plugincatalog.ScanRoot(nil), discoveryRoots...),
		installedRoot:  installedRoot,
		stopPlugin:     stopPlugin,
		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		jobs:           make(chan uninstallJob, 32),
		admission:      tasks.NewQueueAdmission(32),
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

func (s *UninstallService) SetStopPlugin(fn plugins.StopPluginFunc) {
	s.stopPlugin = fn
}

func (s *UninstallService) SetAfterSuccess(fn func(context.Context, string)) {
	s.afterSuccess = fn
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

func (s *UninstallService) Accept(_ context.Context, pluginID string) (string, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", context.Canceled
	}
	if !s.admission.TryAcquire() {
		s.mu.Unlock()
		return "", tasks.ErrQueueFull
	}

	summary := fmt.Sprintf("uninstall plugin: %s", pluginID)
	taskID, err := s.registry.Create("plugin.uninstall", summary)
	if err != nil {
		s.admission.Release()
		s.mu.Unlock()
		return "", err
	}

	runCtx, cancel := context.WithTimeout(s.baseCtx, 5*time.Minute)
	s.cancels[taskID] = cancel
	s.jobs <- uninstallJob{taskID: taskID, pluginID: pluginID, ctx: runCtx}
	s.mu.Unlock()
	return taskID, nil
}

func (s *UninstallService) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.wg.Wait()
		return nil
	}
	s.closed = true
	cancels := make([]context.CancelFunc, 0, len(s.cancels))
	for _, cancel := range s.cancels {
		cancels = append(cancels, cancel)
	}
	s.mu.Unlock()
	s.baseCancel()
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
			s.admission.Release()
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

	if s.stopPlugin != nil {
		s.stopPlugin(job.ctx, job.pluginID)
	}

	if err := job.ctx.Err(); err != nil {
		s.failTask(job.taskID, codePluginUninstallFailed, "插件卸载已取消", "插件卸载已取消")
		return
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(30),
		Summary:  stringPtr("清理数据库记录"),
	})

	if s.repository != nil {
		if err := s.repository.DeleteDesiredState(job.ctx, job.pluginID); err != nil {
			s.logger.Warn("卸载插件 "+job.pluginID+" 时删除启用状态记录失败", "plugin_id", job.pluginID, "err", err.Error())
		}
	}

	if s.packageRepo != nil {
		if err := s.packageRepo.DeletePackageMetadata(job.ctx, job.pluginID); err != nil {
			s.logger.Warn("卸载插件 "+job.pluginID+" 时删除安装包元数据失败", "plugin_id", job.pluginID, "err", err.Error())
		}
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(50),
		Summary:  stringPtr("删除插件安装目录"),
	})

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

	if err := s.refreshCatalog(job.ctx); err != nil {
		s.failTask(job.taskID, codePluginUninstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败")
		return
	}
	if err := job.ctx.Err(); err != nil {
		s.failTask(job.taskID, codePluginUninstallFailed, "插件卸载已取消", "插件卸载已取消")
		return
	}
	if s.afterSuccess != nil {
		s.afterSuccess(job.ctx, job.pluginID)
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

func (s *UninstallService) refreshCatalog(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
		Validator: s.validator,
		Roots:     s.discoveryRoots,
		RepoRoot:  s.repoRoot,
		Logger:    s.logger,
	})
	if err != nil {
		return err
	}

	if packageLoader, ok := s.repository.(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(ctx)
		if err != nil {
			return err
		}
		snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(ctx)
		if err != nil {
			return err
		}
		snapshots = plugins.ApplyDesiredStates(snapshots, states)
	}

	s.catalog.Replace(snapshots)
	return nil
}
