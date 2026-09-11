package lifecycle

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codeInvalidRequest         = errorcodes.PlatformInvalidRequest
	codePlatformTaskTimeout    = errorcodes.PlatformTaskTimeout
	codePluginInstallFailed    = errorcodes.PluginInstallFailed
	codePackageResourceLimit   = errorcodes.PluginPackageResourceLimitExceeded
	codePackageUnsafeEntry     = errorcodes.PluginPackageUnsafeEntry
	codeResourceMissing        = errorcodes.PlatformResourceMissing
	codePluginArtifactInvalid  = errorcodes.PluginArtifactInvalid
	codePluginPlatformMismatch = errorcodes.PluginPlatformMismatch

	maxRemoteDownloadBytes      = 256 * 1024 * 1024
	maxPluginArchiveEntries     = 10_000
	maxPluginArchiveFileBytes   = 64 * 1024 * 1024
	maxPluginArchiveExpandBytes = 512 * 1024 * 1024
	maxPluginArchiveRatio       = 100
	maxPluginDownloadRedirects  = 5
	pluginInspectionTTL         = 10 * time.Minute
	installRenameAttempts       = 10
	installRenameRetryDelay     = 100 * time.Millisecond
)

var errPluginPackageResourceLimit = errors.New("plugin package resource limit exceeded")

type installerDeps struct {
	options      InstallOptions
	now          func() time.Time
	copyDir      func(context.Context, string, string) error
	extractZip   func(context.Context, string, string) (string, error)
	mkdirTemp    func(string, string) (string, error)
	removeAll    func(string) error
	rename       func(string, string) error
	retryRename  func(error) bool
	waitRename   func(context.Context) error
	stat         func(string) (os.FileInfo, error)
	readDir      func(string) ([]os.DirEntry, error)
	downloadFile func(context.Context, string, string) error
}

type InstallService struct {
	operations     *OperationGate
	logger         *slog.Logger
	registry       *tasks.Registry
	catalog        plugins.CatalogStore
	repository     plugins.DesiredStateRepository
	packageRepo    plugins.PackageRepository
	validator      *config.Validator
	repoRoot       string
	discoveryRoots []plugincatalog.ScanRoot
	installedRoot  string
	timeout        time.Duration
	jobs           chan installJob
	admission      *tasks.QueueAdmission

	baseCtx    context.Context
	baseCancel context.CancelFunc

	mu          sync.Mutex
	closed      bool
	cancels     map[string]context.CancelFunc
	inspections map[string]*installInspectionEntry
	deps        installerDeps

	afterSuccess            func(context.Context, string) error
	afterRollback           func(context.Context, string) error
	beforeReplace           plugins.StopPluginFunc
	validateRenderTemplates func(plugins.Snapshot) error
	wg                      sync.WaitGroup
}

type InstallOptions struct {
	Operations              *OperationGate
	AfterSuccess            func(context.Context, string) error
	AfterRollback           func(context.Context, string) error
	BeforeReplace           plugins.StopPluginFunc
	ValidateRenderTemplates func(plugins.Snapshot) error
}

type installJob struct {
	taskID     string
	request    plugins.InstallRequest
	inspection *installInspectionEntry
	ctx        context.Context
}

func NewInstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog plugins.CatalogStore,
	repository plugins.DesiredStateRepository,
	validator *config.Validator,
	repoRoot string,
	discoveryRoots []plugincatalog.ScanRoot,
	timeout time.Duration,
	options InstallOptions,
) (*InstallService, error) {
	if options.Operations == nil {
		return nil, errors.New("plugin install operation gate is required")
	}
	return newInstallService(logger, registry, catalog, repository, validator, repoRoot, discoveryRoots, timeout, installerDeps{options: options})
}

func newInstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog plugins.CatalogStore,
	repository plugins.DesiredStateRepository,
	validator *config.Validator,
	repoRoot string,
	discoveryRoots []plugincatalog.ScanRoot,
	timeout time.Duration,
	deps installerDeps,
) (*InstallService, error) {
	if registry == nil {
		return nil, errors.New("task registry is required")
	}
	if catalog == nil {
		return nil, errors.New("plugin catalog is required")
	}
	if validator == nil {
		return nil, errors.New("plugin validator is required")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	installedRoot, err := installedDiscoveryRoot(discoveryRoots)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	deps = withDefaultInstallerDeps(deps)

	var packageRepo plugins.PackageRepository
	if repo, ok := repository.(plugins.PackageRepository); ok {
		packageRepo = repo
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())
	service := &InstallService{
		operations:              deps.options.Operations,
		afterSuccess:            deps.options.AfterSuccess,
		afterRollback:           deps.options.AfterRollback,
		beforeReplace:           deps.options.BeforeReplace,
		validateRenderTemplates: deps.options.ValidateRenderTemplates,
		logger:                  logger,
		registry:                registry,
		catalog:                 catalog,
		repository:              repository,
		packageRepo:             packageRepo,
		validator:               validator,
		repoRoot:                repoRoot,
		discoveryRoots:          append([]plugincatalog.ScanRoot(nil), discoveryRoots...),
		installedRoot:           installedRoot,
		timeout:                 timeout,
		jobs:                    make(chan installJob, 32),
		admission:               tasks.NewQueueAdmission(32),
		baseCtx:                 baseCtx,
		baseCancel:              baseCancel,
		cancels:                 map[string]context.CancelFunc{},
		inspections:             map[string]*installInspectionEntry{},
		deps:                    deps,
	}

	service.wg.Add(1)
	go service.run()
	return service, nil
}

func (s *InstallService) dropCancel(taskID string) {
	s.mu.Lock()
	cancel := s.cancels[taskID]
	delete(s.cancels, taskID)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *InstallService) Cancel(taskID string) bool {
	snapshot, ok := s.registry.Get(taskID)
	if !ok || snapshot.TaskType != "plugin.install" {
		return false
	}
	if snapshot.Status != tasks.StatusPending && snapshot.Status != tasks.StatusRunning {
		return false
	}

	s.mu.Lock()
	cancel, ok := s.cancels[taskID]
	s.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}

	cancel()
	if snapshot.Status == tasks.StatusPending {
		now := s.deps.now().UTC()
		s.registry.Update(taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusCancelled),
			Summary:    stringPtr("插件安装已取消"),
			FinishedAt: &now,
		})
		s.dropCancel(taskID)
	}

	return true
}

func (s *InstallService) Close() error {
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
	for id, inspection := range s.inspections {
		delete(s.inspections, id)
		inspection.cleanup()
	}
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

func (s *InstallService) run() {
	defer s.wg.Done()

	for {
		select {
		case <-s.baseCtx.Done():
			for {
				select {
				case job := <-s.jobs:
					if job.inspection != nil {
						job.inspection.cleanup()
					}
				default:
					return
				}
			}
		case job := <-s.jobs:
			s.admission.Release()
			s.execute(job)
		}
	}
}

func (s *InstallService) execute(job installJob) {
	defer s.dropCancel(job.taskID)

	snapshot, ok := s.registry.Get(job.taskID)
	if !ok || snapshot.Status == tasks.StatusCancelled {
		if job.inspection != nil {
			job.inspection.cleanup()
		}
		return
	}

	startedAt := s.deps.now().UTC()
	pluginName := "未知插件"
	if job.inspection != nil {
		pluginName = installPluginName(job.inspection.snapshot)
	}
	s.registry.Update(job.taskID, tasks.Update{
		Status:    taskStatusPtr(tasks.StatusRunning),
		Progress:  intPtr(5),
		Summary:   stringPtr("安装插件“" + pluginName + "”"),
		StartedAt: &startedAt,
	})

	err := s.cleanupInstallInspection(job, s.runInstall(job))
	s.reportInstallResult(job, pluginName, err)
}

func installedDiscoveryRoot(discoveryRoots []plugincatalog.ScanRoot) (string, error) {
	for _, root := range discoveryRoots {
		if root.Label == "plugins/installed" {
			return root.Path, nil
		}
	}
	return "", errors.New("plugins/installed discovery root is required")
}

func withDefaultInstallerDeps(deps installerDeps) installerDeps {
	if deps.options.Operations == nil {
		deps.options.Operations = NewOperationGate()
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.copyDir == nil {
		deps.copyDir = copyDirectory
	}
	if deps.extractZip == nil {
		deps.extractZip = extractZipSource
	}
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.rename == nil {
		deps.rename = os.Rename
	}
	if deps.retryRename == nil {
		deps.retryRename = isRetryableInstallRenameError
	}
	if deps.waitRename == nil {
		deps.waitRename = waitForInstallRenameRetry
	}
	if deps.stat == nil {
		deps.stat = os.Stat
	}
	if deps.readDir == nil {
		deps.readDir = os.ReadDir
	}
	if deps.downloadFile == nil {
		deps.downloadFile = downloadHTTPSFile
	}
	return deps
}

func (s *InstallService) refreshCatalog(ctx context.Context, pluginID string) error {
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
		return errors.Join(installError(codePluginInstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败"), err)
	}

	if packageLoader, ok := s.repository.(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(ctx)
		if err != nil {
			return errors.Join(installError(codePluginInstallFailed, "读取插件安装元数据失败", "读取插件安装元数据失败"), err)
		}
		snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(ctx)
		if err != nil {
			return errors.Join(installError(codePluginInstallFailed, "读取插件持久化状态失败", "读取插件持久化状态失败"), err)
		}
		snapshots = plugins.ApplyDesiredStates(snapshots, states)
	}

	s.catalog.RefreshInstalled(snapshots, pluginID)
	return nil
}
