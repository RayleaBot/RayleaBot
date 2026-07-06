package lifecycle

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codeInvalidRequest      = "platform.invalid_request"
	codePlatformTaskTimeout = "platform.task_timeout"
	codePluginInstallFailed = "plugin.install_failed"
	codeResourceMissing     = "platform.resource_missing"

	maxRemoteDownloadBytes = 256 * 1024 * 1024 // 256 MB
)

type installerDeps struct {
	now           func() time.Time
	copyDir       func(context.Context, string, string) error
	extractZip    func(context.Context, string, string) (string, error)
	mkdirTemp     func(string, string) (string, error)
	removeAll     func(string) error
	rename        func(string, string) error
	stat          func(string) (os.FileInfo, error)
	readDir       func(string) ([]os.DirEntry, error)
	hashFile      func(string) (string, error)
	hashDir       func(string) (string, error)
	preparePython func(context.Context, string, []string) error
	prepareNode   func(context.Context, string, []string, bool) error
	downloadFile  func(context.Context, string, string) error
}

type InstallService struct {
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

	baseCtx    context.Context
	baseCancel context.CancelFunc

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	deps    installerDeps

	afterSuccess            func(context.Context, string) error
	validateRenderTemplates func(plugins.Snapshot) error
	wg                      sync.WaitGroup
}

type installJob struct {
	taskID  string
	request plugins.InstallRequest
	ctx     context.Context
}

func executeManagedCommand(ctx context.Context, dir string, env []string, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = append([]string(nil), os.Environ()...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if len(output) != 0 {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return execErr.Err
	}
	return err
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
) (*InstallService, error) {
	return newInstallService(logger, registry, catalog, repository, validator, repoRoot, discoveryRoots, timeout, installerDeps{})
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
	deps = withDefaultInstallerDeps(repoRoot, deps)

	var packageRepo plugins.PackageRepository
	if repo, ok := repository.(plugins.PackageRepository); ok {
		packageRepo = repo
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())
	service := &InstallService{
		logger:         logger,
		registry:       registry,
		catalog:        catalog,
		repository:     repository,
		packageRepo:    packageRepo,
		validator:      validator,
		repoRoot:       repoRoot,
		discoveryRoots: append([]plugincatalog.ScanRoot(nil), discoveryRoots...),
		installedRoot:  installedRoot,
		timeout:        timeout,
		jobs:           make(chan installJob, 32),
		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		cancels:        map[string]context.CancelFunc{},
		deps:           deps,
	}

	service.wg.Add(1)
	go service.run()
	return service, nil
}

type installTaskError struct {
	Code    string
	Message string
	Summary string
}

func (e *installTaskError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func installError(code, message, summary string) error {
	return &installTaskError{
		Code:    code,
		Message: message,
		Summary: summary,
	}
}

func (s *InstallService) failTask(taskID, code, message, summary string) {
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

func (s *InstallService) dropCancel(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cancels, taskID)
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func taskStatusPtr(status tasks.Status) *tasks.Status {
	return &status
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func (s *InstallService) Accept(_ context.Context, request plugins.InstallRequest) (string, error) {
	taskID, err := s.registry.Create("plugin.install", "install plugin from "+request.SourceType+": "+request.Source)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(s.baseCtx, s.timeout)
	s.mu.Lock()
	s.cancels[taskID] = cancel
	s.mu.Unlock()

	select {
	case s.jobs <- installJob{taskID: taskID, request: request, ctx: runCtx}:
		return taskID, nil
	case <-s.baseCtx.Done():
		cancel()
		s.registry.Update(taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusFailed),
			FinishedAt: timePtr(s.deps.now().UTC()),
			Summary:    stringPtr("后台安装执行器不可用"),
			Error: &tasks.ErrorSummary{
				Code:    "platform.internal_error",
				Message: "安装执行器不可用",
			},
		})
		return "", errors.New("install service is shutting down")
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

func (s *InstallService) SetAfterSuccess(fn func(context.Context, string) error) {
	if s == nil {
		return
	}
	s.afterSuccess = fn
}

func (s *InstallService) SetRenderTemplateValidator(fn func(plugins.Snapshot) error) {
	if s == nil {
		return
	}
	s.validateRenderTemplates = fn
}

func (s *InstallService) Close() error {
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

func (s *InstallService) run() {
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

func (s *InstallService) execute(job installJob) {
	defer s.dropCancel(job.taskID)

	snapshot, ok := s.registry.Get(job.taskID)
	if !ok {
		return
	}
	if snapshot.Status == tasks.StatusCancelled {
		return
	}

	startedAt := s.deps.now().UTC()
	s.registry.Update(job.taskID, tasks.Update{
		Status:    taskStatusPtr(tasks.StatusRunning),
		Progress:  intPtr(5),
		Summary:   stringPtr("准备安装源"),
		StartedAt: &startedAt,
	})

	err := s.runInstall(job)
	switch {
	case err == nil:
		now := s.deps.now().UTC()
		s.registry.Update(job.taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusSucceeded),
			Progress:   intPtr(100),
			Summary:    stringPtr("插件安装完成"),
			FinishedAt: &now,
			Result: &tasks.ResultSummary{
				Summary: "插件已安装并刷新插件目录索引",
			},
		})
	case errors.Is(err, context.Canceled):
		now := s.deps.now().UTC()
		s.registry.Update(job.taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusCancelled),
			Summary:    stringPtr("插件安装已取消"),
			FinishedAt: &now,
		})
	case errors.Is(err, context.DeadlineExceeded):
		s.failTask(job.taskID, codePlatformTaskTimeout, "插件安装超时", "插件安装超时")
	default:
		var installErr *installTaskError
		if errors.As(err, &installErr) {
			s.failTask(job.taskID, installErr.Code, installErr.Message, installErr.Summary)
			return
		}
		s.failTask(job.taskID, codePluginInstallFailed, "插件安装失败", "插件安装失败")
	}
}

func installedDiscoveryRoot(discoveryRoots []plugincatalog.ScanRoot) (string, error) {
	for _, root := range discoveryRoots {
		if root.Label == "plugins/installed" {
			return root.Path, nil
		}
	}
	return "", errors.New("plugins/installed discovery root is required")
}

func withDefaultInstallerDeps(repoRoot string, deps installerDeps) installerDeps {
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
	if deps.stat == nil {
		deps.stat = os.Stat
	}
	if deps.readDir == nil {
		deps.readDir = os.ReadDir
	}
	if deps.hashFile == nil {
		deps.hashFile = hashFileSHA256
	}
	if deps.hashDir == nil {
		deps.hashDir = hashDirectorySHA256
	}
	if deps.preparePython == nil {
		deps.preparePython = func(ctx context.Context, pluginDir string, dependencies []string) error {
			return preparePythonEnvironment(ctx, repoRoot, pluginDir, dependencies)
		}
	}
	if deps.prepareNode == nil {
		deps.prepareNode = func(ctx context.Context, pluginDir string, dependencies []string, allowInstallScripts bool) error {
			return prepareNodeEnvironment(ctx, repoRoot, pluginDir, dependencies, allowInstallScripts)
		}
	}
	if deps.downloadFile == nil {
		deps.downloadFile = downloadHTTPSFile
	}
	return deps
}

func (s *InstallService) refreshCatalog(ctx context.Context) error {
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
		return installError(codePluginInstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败")
	}

	if packageLoader, ok := s.repository.(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(ctx)
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件安装元数据失败", "读取插件安装元数据失败")
		}
		snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(ctx)
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件持久化状态失败", "读取插件持久化状态失败")
		}
		snapshots = plugins.ApplyDesiredStates(snapshots, states)
	}

	s.catalog.Replace(snapshots)
	return nil
}

func (s *InstallService) prepareDependencies(ctx context.Context, candidateDir string, snapshot plugins.Snapshot, allowInstallScripts bool) error {
	if snapshot.RequireInstallScripts && !allowInstallScripts {
		return installError("platform.install_script_blocked", "插件安装脚本被默认安全策略阻止", "插件安装脚本被默认安全策略阻止")
	}

	switch snapshot.Runtime {
	case "python":
		if err := s.deps.preparePython(ctx, candidateDir, snapshot.PythonDependencies); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return installError(codePluginInstallFailed, "准备 Python 插件依赖环境失败", "准备 Python 插件依赖环境失败")
		}
	case "nodejs":
		needsNodeSetup := len(snapshot.NodeDependencies) > 0 || snapshot.RequireInstallScripts
		if !needsNodeSetup {
			return nil
		}
		if snapshot.RequireInstallScripts {
			packageJSONPath := filepath.Join(candidateDir, "package.json")
			if _, err := s.deps.stat(packageJSONPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return installError(codePluginInstallFailed, "插件声明需要安装脚本但 package.json 缺失", "插件声明需要安装脚本但 package.json 缺失")
				}
				return installError(codePluginInstallFailed, "检查 Node.js 插件 package.json 失败", "检查 Node.js 插件 package.json 失败")
			}
		}
		allowNodeScripts := allowInstallScripts && snapshot.RequireInstallScripts
		if err := s.deps.prepareNode(ctx, candidateDir, snapshot.NodeDependencies, allowNodeScripts); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return installError(codePluginInstallFailed, "准备 Node.js 插件依赖环境失败", "准备 Node.js 插件依赖环境失败")
		}
	}

	return nil
}

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("invalid HTTPS URL: %s", rawURL)
	}

	client := &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote server returned HTTP %d", resp.StatusCode)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	limitedReader := io.LimitReader(resp.Body, maxRemoteDownloadBytes+1)
	written, err := io.Copy(outFile, limitedReader)
	if err != nil {
		return err
	}
	if written > maxRemoteDownloadBytes {
		return fmt.Errorf("download exceeded maximum size of %d bytes", maxRemoteDownloadBytes)
	}
	return nil
}

func (s *InstallService) runInstall(job installJob) error {
	workingRoot, candidateDir, cleanup, err := s.prepareSource(job.ctx, job.request)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := job.ctx.Err(); err != nil {
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(20),
		Summary:  stringPtr("校验插件 manifest"),
	})

	candidateSnapshot, err := s.loadCandidateSnapshot(candidateDir)
	if err != nil {
		return err
	}
	metadata, err := s.buildPackageMetadata(job.request, candidateSnapshot, candidateDir)
	if err != nil {
		return err
	}
	if _, exists := s.catalog.Get(candidateSnapshot.PluginID); exists {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	}
	if s.validateRenderTemplates != nil {
		if err := s.validateRenderTemplates(candidateSnapshot); err != nil {
			return installError(codePluginInstallFailed, err.Error(), "插件渲染模板校验失败")
		}
	}

	if err := job.ctx.Err(); err != nil {
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(40),
		Summary:  stringPtr("准备插件依赖环境"),
	})

	if err := s.prepareDependencies(job.ctx, candidateDir, candidateSnapshot, job.request.AllowInstallScripts); err != nil {
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(60),
		Summary:  stringPtr("写入正式安装目录"),
	})

	if err := os.MkdirAll(s.installedRoot, 0o755); err != nil {
		return installError(codePluginInstallFailed, "创建插件安装目录失败", "创建插件安装目录失败")
	}

	finalTarget := filepath.Join(s.installedRoot, candidateSnapshot.PluginID)
	if _, err := s.deps.stat(finalTarget); err == nil {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	} else if !errors.Is(err, os.ErrNotExist) {
		return installError(codePluginInstallFailed, "检查插件安装目录失败", "检查插件安装目录失败")
	}

	if err := s.deps.rename(candidateDir, finalTarget); err != nil {
		return installError(codePluginInstallFailed, "写入插件安装目录失败", "写入插件安装目录失败")
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(75),
		Summary:  stringPtr("刷新插件目录索引"),
	})

	if err := s.refreshCatalog(job.ctx); err != nil {
		_ = s.deps.removeAll(finalTarget)
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(90),
		Summary:  stringPtr("写入安装元数据"),
	})

	if s.packageRepo != nil {
		metadata.InstalledAt = s.deps.now().UTC()
		if err := s.packageRepo.SavePackageMetadata(job.ctx, metadata); err != nil {
			_ = s.deps.removeAll(finalTarget)
			_ = s.refreshCatalog(job.ctx)
			return installError(codePluginInstallFailed, "写入插件安装元数据失败", "写入插件安装元数据失败")
		}
	}
	if s.afterSuccess != nil {
		if err := s.afterSuccess(job.ctx, candidateSnapshot.PluginID); err != nil {
			if s.packageRepo != nil {
				_ = s.packageRepo.DeletePackageMetadata(job.ctx, candidateSnapshot.PluginID)
			}
			_ = s.deps.removeAll(finalTarget)
			_ = s.refreshCatalog(job.ctx)
			return installError(codePluginInstallFailed, err.Error(), "插件安装后处理失败")
		}
	}

	_ = workingRoot
	return nil
}

func (s *InstallService) loadCandidateSnapshot(candidateDir string) (plugins.Snapshot, error) {
	infoPath := filepath.Join(candidateDir, "info.json")
	snapshot, ok, err := plugincatalog.LoadSnapshot(infoPath, "plugins/installed", s.repoRoot, s.validator, plugins.ManifestValidationMaxSummary, s.logger)
	if err != nil {
		return plugins.Snapshot{}, installError(codePluginInstallFailed, "读取插件 manifest 失败", "读取插件 manifest 失败")
	}
	if !ok {
		return plugins.Snapshot{}, installError(codeInvalidRequest, "插件 manifest 缺少必需字段", "插件 manifest 缺少必需字段")
	}
	if !snapshot.Valid {
		return plugins.Snapshot{}, installError(codePluginInstallFailed, snapshot.ValidationSummary, "插件 manifest 校验失败")
	}
	if snapshot.PluginID == "" {
		return plugins.Snapshot{}, installError(codeInvalidRequest, "插件 manifest 缺少插件 ID", "插件 manifest 缺少插件 ID")
	}
	return snapshot, nil
}

func (s *InstallService) buildPackageMetadata(request plugins.InstallRequest, snapshot plugins.Snapshot, candidateDir string) (plugins.PackageMetadata, error) {
	manifestHash, err := s.deps.hashFile(filepath.Join(candidateDir, "info.json"))
	if err != nil {
		return plugins.PackageMetadata{}, installError(codePluginInstallFailed, "计算插件 manifest 哈希失败", "计算插件 manifest 哈希失败")
	}
	packageHash, err := s.deps.hashDir(candidateDir)
	if err != nil {
		return plugins.PackageMetadata{}, installError(codePluginInstallFailed, "计算插件安装包哈希失败", "计算插件安装包哈希失败")
	}

	return plugins.PackageMetadata{
		PluginID:     snapshot.PluginID,
		SourceType:   request.SourceType,
		SourceRef:    request.Source,
		Version:      snapshot.Version,
		ManifestHash: manifestHash,
		PackageHash:  packageHash,
	}, nil
}

func (s *InstallService) prepareSource(ctx context.Context, request plugins.InstallRequest) (string, string, func(), error) {
	if err := os.MkdirAll(s.installedRoot, 0o755); err != nil {
		return "", "", func() {}, installError(codePluginInstallFailed, "创建插件安装目录失败", "创建插件安装目录失败")
	}
	tempRoot, err := s.deps.mkdirTemp(s.installedRoot, ".plugin-install-*")
	if err != nil {
		return "", "", func() {}, installError(codePluginInstallFailed, "创建安装临时目录失败", "创建安装临时目录失败")
	}

	cleanup := func() {
		_ = s.deps.removeAll(tempRoot)
	}

	switch request.SourceType {
	case "local_directory":
		info, err := s.deps.stat(request.Source)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cleanup()
				return "", "", func() {}, installError(codeResourceMissing, "插件来源目录不存在", "插件来源目录不存在")
			}
			cleanup()
			return "", "", func() {}, installError(codePluginInstallFailed, "检查插件来源目录失败", "检查插件来源目录失败")
		}
		if !info.IsDir() {
			cleanup()
			return "", "", func() {}, installError(codeInvalidRequest, "插件来源必须是目录", "插件来源必须是目录")
		}

		candidate := filepath.Join(tempRoot, "candidate")
		if err := s.deps.copyDir(ctx, request.Source, candidate); err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, installError(codePluginInstallFailed, "复制插件来源目录失败", "复制插件来源目录失败")
		}
		return tempRoot, candidate, cleanup, nil
	case "local_zip":
		info, err := s.deps.stat(request.Source)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cleanup()
				return "", "", func() {}, installError(codeResourceMissing, "插件来源压缩包不存在", "插件来源压缩包不存在")
			}
			cleanup()
			return "", "", func() {}, installError(codePluginInstallFailed, "检查插件来源压缩包失败", "检查插件来源压缩包失败")
		}
		if info.IsDir() {
			cleanup()
			return "", "", func() {}, installError(codeInvalidRequest, "插件来源必须是压缩包文件", "插件来源必须是压缩包文件")
		}

		candidate, err := s.deps.extractZip(ctx, request.Source, tempRoot)
		if err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, err
		}
		return tempRoot, candidate, cleanup, nil
	case "remote_url":
		parsed, err := url.Parse(request.Source)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			cleanup()
			return "", "", func() {}, installError(codeInvalidRequest, "远程来源必须是 HTTPS URL", "远程来源必须是 HTTPS URL")
		}

		downloadPath := filepath.Join(tempRoot, "download.zip")
		if err := s.deps.downloadFile(ctx, request.Source, downloadPath); err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, installError(codePluginInstallFailed, "下载远程插件压缩包失败", "下载远程插件压缩包失败")
		}

		candidate, err := s.deps.extractZip(ctx, downloadPath, tempRoot)
		if err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, err
		}
		return tempRoot, candidate, cleanup, nil
	default:
		cleanup()
		return "", "", func() {}, installError(codeInvalidRequest, "插件来源类型不受支持", "插件来源类型不受支持")
	}
}

func copyDirectory(ctx context.Context, sourceRoot, targetRoot string) error {
	info, err := os.Stat(sourceRoot)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", sourceRoot)
	}
	if err := os.MkdirAll(targetRoot, info.Mode().Perm()); err != nil {
		return err
	}

	return filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceRoot {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink entries are not supported in install sources")
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetRoot, relativePath)

		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		return copyFile(path, targetPath)
	})
}

func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return nil
}

func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func hashDirectorySHA256(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", root)
	}

	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, relativePath)
		return nil
	}); err != nil {
		return "", err
	}

	sort.Strings(files)
	hasher := sha256.New()
	for _, relativePath := range files {
		if _, err := io.WriteString(hasher, filepath.ToSlash(relativePath)); err != nil {
			return "", err
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return "", err
		}

		file, err := os.Open(filepath.Join(root, relativePath))
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hasher, file); err != nil {
			file.Close()
			return "", err
		}
		file.Close()
		if _, err := hasher.Write([]byte{0}); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func extractZipSource(ctx context.Context, archivePath, tempRoot string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", installError(codePluginInstallFailed, "解压插件压缩包失败", "解压插件压缩包失败")
	}
	defer reader.Close()

	extractRoot := filepath.Join(tempRoot, "unzipped")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", installError(codePluginInstallFailed, "创建解压临时目录失败", "创建解压临时目录失败")
	}

	topLevels := map[string]struct{}{}

	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		cleanName := filepath.Clean(file.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			return "", installError(codePluginInstallFailed, "插件压缩包包含越界路径", "插件压缩包包含越界路径")
		}

		targetPath := filepath.Join(extractRoot, cleanName)
		relativePath, err := filepath.Rel(extractRoot, targetPath)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return "", installError(codePluginInstallFailed, "插件压缩包包含越界路径", "插件压缩包包含越界路径")
		}

		parts := strings.Split(filepath.ToSlash(cleanName), "/")
		if len(parts) > 0 && parts[0] != "." && parts[0] != "" {
			topLevels[parts[0]] = struct{}{}
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, normalizedZipEntryMode(file)); err != nil {
				return "", installError(codePluginInstallFailed, "创建解压目录失败", "创建解压目录失败")
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", installError(codePluginInstallFailed, "创建解压目录失败", "创建解压目录失败")
		}

		readerHandle, err := file.Open()
		if err != nil {
			return "", installError(codePluginInstallFailed, "读取压缩包条目失败", "读取压缩包条目失败")
		}

		targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, normalizedZipEntryMode(file))
		if err != nil {
			readerHandle.Close()
			return "", installError(codePluginInstallFailed, "写入解压文件失败", "写入解压文件失败")
		}

		if _, err := io.Copy(targetFile, readerHandle); err != nil {
			targetFile.Close()
			readerHandle.Close()
			return "", installError(codePluginInstallFailed, "写入解压文件失败", "写入解压文件失败")
		}

		targetFile.Close()
		readerHandle.Close()
	}

	if len(topLevels) != 1 {
		return "", installError(codePluginInstallFailed, "压缩包必须只包含一个插件根目录", "压缩包必须只包含一个插件根目录")
	}

	var rootName string
	for name := range topLevels {
		rootName = name
	}

	rootPath := filepath.Join(extractRoot, filepath.FromSlash(rootName))
	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		return "", installError(codePluginInstallFailed, "压缩包必须只包含一个插件根目录", "压缩包必须只包含一个插件根目录")
	}
	return rootPath, nil
}

func normalizedZipEntryMode(file *zip.File) os.FileMode {
	mode := file.Mode().Perm()
	if file.FileInfo().IsDir() {
		if mode&0o111 == 0 {
			return 0o755
		}
		return mode
	}
	if mode == 0 {
		return 0o644
	}
	return mode
}

type runtimeResolver interface {
	ResolveEntrypoint(context.Context, string, string) (string, error)
}

var newRuntimeResolver = func(repoRoot string) runtimeResolver {
	return deps.NewRuntime(repoRoot)
}

var runManagedCommand = executeManagedCommand

func preparePythonEnvironment(ctx context.Context, repoRoot string, pluginDir string, dependencies []string) error {
	if len(dependencies) == 0 {
		return nil
	}

	resolver := newRuntimeResolver(repoRoot)
	pythonExecutable, err := resolver.ResolveEntrypoint(ctx, "python-runtime", "python")
	if err != nil {
		return err
	}

	venvDir := filepath.Join(pluginDir, ".venv")
	if err := runManagedCommand(ctx, pluginDir, nil, pythonExecutable, "-m", "venv", venvDir); err != nil {
		return err
	}

	venvPython, err := virtualenvPythonExecutable(venvDir)
	if err != nil {
		return err
	}
	args := append([]string{"-m", "pip", "install"}, dependencies...)
	return runManagedCommand(ctx, pluginDir, nil, venvPython, args...)
}

func prepareNodeEnvironment(ctx context.Context, repoRoot string, pluginDir string, dependencies []string, allowInstallScripts bool) error {
	packageJSONPath := filepath.Join(pluginDir, "package.json")
	_, err := os.Stat(packageJSONPath)
	hasPackageJSON := err == nil

	if len(dependencies) == 0 && !hasPackageJSON {
		return nil
	}

	resolver := newRuntimeResolver(repoRoot)
	npmExecutable, err := resolver.ResolveEntrypoint(ctx, "nodejs-runtime", "npm")
	if err != nil {
		return err
	}

	userConfigPath := filepath.Join(pluginDir, ".npmrc.managed")
	if err := os.WriteFile(userConfigPath, nil, 0o644); err != nil {
		return err
	}

	args := buildNodeInstallArgs(pluginDir, dependencies, allowInstallScripts, hasPackageJSON)
	env := []string{"NPM_CONFIG_USERCONFIG=" + userConfigPath}
	return runManagedCommand(ctx, pluginDir, env, npmExecutable, args...)
}

func buildNodeInstallArgs(pluginDir string, dependencies []string, allowInstallScripts bool, hasPackageJSON bool) []string {
	args := []string{}
	hasPackageLock := false
	if hasPackageJSON {
		for _, name := range []string{"package-lock.json", "npm-shrinkwrap.json"} {
			if _, err := os.Stat(filepath.Join(pluginDir, name)); err == nil {
				hasPackageLock = true
				break
			}
		}
	}
	if hasPackageLock {
		args = append(args, "ci")
	} else {
		args = append(args, "install")
	}
	if !allowInstallScripts {
		args = append(args, "--ignore-scripts")
	}
	if !hasPackageJSON {
		args = append(args, dependencies...)
	}
	return args
}

func virtualenvPythonExecutable(venvDir string) (string, error) {
	candidates := []string{
		filepath.Join(venvDir, "bin", "python"),
		filepath.Join(venvDir, "bin", "python3"),
		filepath.Join(venvDir, "Scripts", "python.exe"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("virtualenv python executable is missing under %s", venvDir)
}
