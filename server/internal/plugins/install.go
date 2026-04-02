package plugins

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/tasks"
)

const (
	codePlatformTaskTimeout = "platform.task_timeout"
	codePluginInstallFailed = "plugin.install_failed"
)

type InstallRequest struct {
	SourceType          string
	Source              string
	AllowInstallScripts bool
}

type InstallCoordinator interface {
	Accept(context.Context, InstallRequest) (string, error)
	Cancel(string) bool
	Close() error
}

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
	catalog        *Catalog
	repository     DesiredStateRepository
	packageRepo    PackageRepository
	validator      *schema.Validator
	repoRoot       string
	discoveryRoots []ScanRoot
	installedRoot  string
	timeout        time.Duration
	jobs           chan installJob

	baseCtx    context.Context
	baseCancel context.CancelFunc
	wg         sync.WaitGroup

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	deps    installerDeps
}

type installJob struct {
	taskID  string
	request InstallRequest
	ctx     context.Context
}

func NewInstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog *Catalog,
	repository DesiredStateRepository,
	validator *schema.Validator,
	repoRoot string,
	discoveryRoots []ScanRoot,
	timeout time.Duration,
) (*InstallService, error) {
	return newInstallService(logger, registry, catalog, repository, validator, repoRoot, discoveryRoots, timeout, installerDeps{})
}

func newInstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog *Catalog,
	repository DesiredStateRepository,
	validator *schema.Validator,
	repoRoot string,
	discoveryRoots []ScanRoot,
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

	if timeout <= 0 {
		timeout = 15 * time.Minute
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

	var packageRepo PackageRepository
	if repo, ok := repository.(PackageRepository); ok {
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
		discoveryRoots: append([]ScanRoot(nil), discoveryRoots...),
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

func (s *InstallService) Accept(_ context.Context, request InstallRequest) (string, error) {
	taskID, err := s.registry.Create("plugin.install", fmt.Sprintf("install plugin from %s: %s", request.SourceType, request.Source))
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

	// The candidate directory has been moved out of the working root.
	candidateDir = ""

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(75),
		Summary:  stringPtr("刷新插件目录索引"),
	})

	if err := s.refreshCatalog(); err != nil {
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
			_ = s.refreshCatalog()
			return installError(codePluginInstallFailed, "写入插件安装元数据失败", "写入插件安装元数据失败")
		}
	}

	_ = workingRoot
	return nil
}

func (s *InstallService) buildPackageMetadata(request InstallRequest, snapshot Snapshot, candidateDir string) (PackageMetadata, error) {
	manifestHash, err := s.deps.hashFile(filepath.Join(candidateDir, "info.json"))
	if err != nil {
		return PackageMetadata{}, installError(codePluginInstallFailed, "计算插件 manifest 哈希失败", "计算插件 manifest 哈希失败")
	}
	packageHash, err := s.deps.hashDir(candidateDir)
	if err != nil {
		return PackageMetadata{}, installError(codePluginInstallFailed, "计算插件安装包哈希失败", "计算插件安装包哈希失败")
	}

	return PackageMetadata{
		PluginID:     snapshot.PluginID,
		SourceType:   request.SourceType,
		SourceRef:    request.Source,
		Version:      snapshot.Version,
		ManifestHash: manifestHash,
		PackageHash:  packageHash,
	}, nil
}

func (s *InstallService) prepareDependencies(ctx context.Context, candidateDir string, snapshot Snapshot, allowInstallScripts bool) error {
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

func (s *InstallService) prepareSource(ctx context.Context, request InstallRequest) (string, string, func(), error) {
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

func (s *InstallService) loadCandidateSnapshot(candidateDir string) (Snapshot, error) {
	infoPath := filepath.Join(candidateDir, "info.json")
	snapshot, ok, err := loadSnapshot(infoPath, "plugins/installed", s.repoRoot, s.validator, validationMaxSummary, s.logger)
	if err != nil {
		return Snapshot{}, installError(codePluginInstallFailed, "读取插件 manifest 失败", "读取插件 manifest 失败")
	}
	if !ok {
		return Snapshot{}, installError(codeInvalidRequest, "插件 manifest 缺少必需字段", "插件 manifest 缺少必需字段")
	}
	if !snapshot.Valid {
		return Snapshot{}, installError(codePluginInstallFailed, snapshot.ValidationSummary, "插件 manifest 校验失败")
	}
	if snapshot.PluginID == "" {
		return Snapshot{}, installError(codeInvalidRequest, "插件 manifest 缺少插件 ID", "插件 manifest 缺少插件 ID")
	}
	return snapshot, nil
}

func (s *InstallService) refreshCatalog() error {
	snapshots, _, err := Discover(DiscoverOptions{
		Validator: s.validator,
		Roots:     s.discoveryRoots,
		RepoRoot:  s.repoRoot,
		Logger:    s.logger,
	})
	if err != nil {
		return installError(codePluginInstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败")
	}

	reloaded := NewCatalog(snapshots)
	if packageLoader, ok := s.repository.(PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件安装元数据失败", "读取插件安装元数据失败")
		}
		reloaded.Replace(ApplyPackageMetadata(reloaded.List(), packageMetadata))
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(context.Background())
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件持久化状态失败", "读取插件持久化状态失败")
		}
		reloaded.ApplyDesiredStates(states)
	}

	s.catalog.Replace(reloaded.List())
	return nil
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

const maxRemoteDownloadBytes = 256 * 1024 * 1024 // 256 MB

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
		return fmt.Errorf("remote file exceeds maximum size of %d bytes", maxRemoteDownloadBytes)
	}

	return nil
}
