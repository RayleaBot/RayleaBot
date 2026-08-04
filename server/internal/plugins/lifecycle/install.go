package lifecycle

import (
	"archive/zip"
	"context"
	"crypto/rand"
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
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codeInvalidRequest         = "platform.invalid_request"
	codePlatformTaskTimeout    = "platform.task_timeout"
	codePluginInstallFailed    = "plugin.install_failed"
	codePackageResourceLimit   = "plugin.package_resource_limit_exceeded"
	codePackageUnsafeEntry     = "plugin.package_unsafe_entry"
	codeResourceMissing        = "platform.resource_missing"
	codePluginArtifactInvalid  = "plugin.artifact_invalid"
	codePluginPlatformMismatch = "plugin.platform_mismatch"

	maxRemoteDownloadBytes      = 256 * 1024 * 1024
	maxPluginArchiveEntries     = 10_000
	maxPluginArchiveFileBytes   = 64 * 1024 * 1024
	maxPluginArchiveExpandBytes = 512 * 1024 * 1024
	maxPluginArchiveRatio       = 100
	maxPluginDownloadRedirects  = 5
	pluginInspectionTTL         = 10 * time.Minute
)

var errPluginPackageResourceLimit = errors.New("plugin package resource limit exceeded")

type installerDeps struct {
	now          func() time.Time
	copyDir      func(context.Context, string, string) error
	extractZip   func(context.Context, string, string) (string, error)
	mkdirTemp    func(string, string) (string, error)
	removeAll    func(string) error
	rename       func(string, string) error
	stat         func(string) (os.FileInfo, error)
	readDir      func(string) ([]os.DirEntry, error)
	hashFile     func(string) (string, error)
	hashDir      func(string) (string, error)
	downloadFile func(context.Context, string, string) error
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
	admission      *tasks.QueueAdmission

	baseCtx    context.Context
	baseCancel context.CancelFunc

	mu          sync.Mutex
	closed      bool
	cancels     map[string]context.CancelFunc
	inspections map[string]*installInspectionEntry
	deps        installerDeps

	afterSuccess            func(context.Context, string) error
	afterRollback           func(context.Context, string)
	beforeReplace           plugins.StopPluginFunc
	validateRenderTemplates func(plugins.Snapshot) error
	wg                      sync.WaitGroup
}

type installJob struct {
	taskID     string
	request    plugins.InstallRequest
	inspection *installInspectionEntry
	ctx        context.Context
}

type installInspectionEntry struct {
	inspection   plugins.InstallInspection
	request      plugins.InstallRequest
	workingRoot  string
	candidateDir string
	cleanup      func()
	snapshot     plugins.Snapshot
	metadata     plugins.PackageMetadata
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
		admission:      tasks.NewQueueAdmission(32),
		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		cancels:        map[string]context.CancelFunc{},
		inspections:    map[string]*installInspectionEntry{},
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

func InstallErrorCode(err error) string {
	var installErr *installTaskError
	if errors.As(err, &installErr) {
		return installErr.Code
	}
	return ""
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
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", context.Canceled
	}
	if !request.TrustedCodeConfirmed {
		s.mu.Unlock()
		return "", plugins.ErrTrustedCodeConfirmation
	}
	entry, err := s.consumeInspectionLocked(request)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	if !s.admission.TryAcquire() {
		s.mu.Unlock()
		return "", tasks.ErrQueueFull
	}

	taskID, err := s.registry.Create("plugin.install", "install plugin from "+request.SourceType+": "+request.Source)
	if err != nil {
		s.admission.Release()
		s.mu.Unlock()
		return "", err
	}

	runCtx, cancel := context.WithTimeout(s.baseCtx, s.timeout)
	s.cancels[taskID] = cancel
	delete(s.inspections, request.InspectionID)
	s.jobs <- installJob{taskID: taskID, request: request, inspection: entry, ctx: runCtx}
	s.mu.Unlock()
	return taskID, nil
}

func (s *InstallService) Inspect(ctx context.Context, request plugins.InstallRequest) (plugins.InstallInspection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return plugins.InstallInspection{}, context.Canceled
	}
	s.cleanupExpiredInspectionsLocked(s.deps.now().UTC())
	s.mu.Unlock()

	workingRoot, candidateDir, cleanup, err := s.prepareSource(ctx, request)
	if err != nil {
		return plugins.InstallInspection{}, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			cleanup()
		}
	}()

	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		return plugins.InstallInspection{}, installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
	}
	verified, err := artifact.Verify(candidateDir, artifact.Options{ExpectedPlatform: targetPlatform})
	if err != nil {
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return plugins.InstallInspection{}, installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
		}
		return plugins.InstallInspection{}, installError(codePluginArtifactInvalid, err.Error(), "插件 artifact 校验失败")
	}
	if request.ExpectedManifestSHA256 != "" && verified.Document.ManifestSHA256 != request.ExpectedManifestSHA256 {
		return plugins.InstallInspection{}, installError("plugin.store_integrity_mismatch", "插件 manifest 摘要与商店目录不一致", "插件商店产物完整性校验失败")
	}
	snapshot, err := s.loadCandidateSnapshot(candidateDir)
	if err != nil {
		return plugins.InstallInspection{}, err
	}
	metadata, err := s.buildPackageMetadata(request, snapshot, candidateDir)
	if err != nil {
		return plugins.InstallInspection{}, err
	}
	id, err := newInspectionID()
	if err != nil {
		return plugins.InstallInspection{}, installError(codePluginInstallFailed, "生成插件检查标识失败", "生成插件检查标识失败")
	}
	now := s.deps.now().UTC()
	inspection := plugins.InstallInspection{
		InspectionID:   id,
		ExpiresAt:      now.Add(pluginInspectionTTL),
		PackageSHA256:  metadata.PackageHash,
		SourceType:     request.SourceType,
		Source:         request.Source,
		PluginID:       snapshot.PluginID,
		PluginName:     snapshot.Name,
		Version:        snapshot.Version,
		Author:         snapshot.Author,
		License:        snapshot.License,
		SourceLabel:    installSourceLabel(request),
		Capabilities:   append([]string(nil), snapshot.DeclaredCapabilities...),
		TargetPlatform: verified.Document.TargetPlatform,
		Artifact: plugins.ArtifactInspection{
			Valid:          true,
			Version:        verified.Document.ArtifactVersion,
			ManifestSHA256: verified.Document.ManifestSHA256,
			FileCount:      len(verified.Document.Files),
		},
	}
	for _, file := range verified.Document.Files {
		switch file.Role {
		case "backend":
			inspection.Backend = plugins.InstallBackendInspection{Entry: snapshot.Entry, Path: file.Path, Size: file.Size, SHA256: file.SHA256}
		case "ui":
			inspection.UI.FileCount++
		}
	}
	inspection.UI.Enabled = verified.UIAvailable
	if len(verified.UIEntries) > 0 {
		inspection.UI.Entry = verified.UIEntries[0]
	}
	entry := &installInspectionEntry{
		inspection:   inspection,
		request:      request,
		workingRoot:  workingRoot,
		candidateDir: candidateDir,
		cleanup:      cleanup,
		snapshot:     snapshot,
		metadata:     metadata,
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return plugins.InstallInspection{}, context.Canceled
	}
	s.inspections[id] = entry
	s.mu.Unlock()
	succeeded = true
	return inspection, nil
}

func (s *InstallService) consumeInspectionLocked(request plugins.InstallRequest) (*installInspectionEntry, error) {
	id := strings.TrimSpace(request.InspectionID)
	if id == "" || strings.TrimSpace(request.PackageSHA256) == "" {
		return nil, plugins.ErrInstallInspectionRequired
	}
	entry, ok := s.inspections[id]
	if !ok {
		return nil, plugins.ErrInstallInspectionRequired
	}
	if !entry.inspection.ExpiresAt.After(s.deps.now().UTC()) {
		delete(s.inspections, id)
		entry.cleanup()
		return nil, plugins.ErrInstallInspectionExpired
	}
	if request.PackageSHA256 != entry.inspection.PackageSHA256 || !sameInstallRequestIdentity(request, entry.request) {
		return nil, plugins.ErrInstallDigestMismatch
	}
	return entry, nil
}

func sameInstallRequestIdentity(left, right plugins.InstallRequest) bool {
	return left.SourceType == right.SourceType &&
		left.Source == right.Source &&
		left.ResolvedSourceType == right.ResolvedSourceType &&
		left.ResolvedSource == right.ResolvedSource &&
		left.ExpectedArchiveSize == right.ExpectedArchiveSize &&
		left.ExpectedArchiveSHA256 == right.ExpectedArchiveSHA256 &&
		left.ExpectedManifestSHA256 == right.ExpectedManifestSHA256 &&
		left.ReplaceExisting == right.ReplaceExisting &&
		left.PublisherID == right.PublisherID &&
		left.PublisherName == right.PublisherName &&
		left.PublisherVerified == right.PublisherVerified &&
		left.CatalogDigest == right.CatalogDigest
}

func (s *InstallService) cleanupExpiredInspectionsLocked(now time.Time) {
	for id, entry := range s.inspections {
		if entry.inspection.ExpiresAt.After(now) {
			continue
		}
		delete(s.inspections, id)
		entry.cleanup()
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
	s.afterSuccess = fn
}

func (s *InstallService) SetBeforeReplace(fn plugins.StopPluginFunc) {
	s.beforeReplace = fn
}

func (s *InstallService) SetAfterRollback(fn func(context.Context, string)) {
	s.afterRollback = fn
}

func (s *InstallService) SetRenderTemplateValidator(fn func(plugins.Snapshot) error) {
	s.validateRenderTemplates = fn
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
	if job.inspection != nil {
		defer job.inspection.cleanup()
	}

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

func withDefaultInstallerDeps(_ string, deps installerDeps) installerDeps {
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

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("invalid HTTPS URL: %s", rawURL)
	}

	client := &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
		CheckRedirect: validatePluginDownloadRedirect,
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
	if resp.ContentLength > maxRemoteDownloadBytes {
		return fmt.Errorf("%w: download exceeded maximum size of %d bytes", errPluginPackageResourceLimit, maxRemoteDownloadBytes)
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
		return fmt.Errorf("%w: download exceeded maximum size of %d bytes", errPluginPackageResourceLimit, maxRemoteDownloadBytes)
	}
	return nil
}

func validatePluginDownloadRedirect(request *http.Request, via []*http.Request) error {
	if len(via) > maxPluginDownloadRedirects {
		return errors.New("remote plugin download exceeded redirect limit")
	}
	if request == nil || request.URL == nil || request.URL.Scheme != "https" || request.URL.Host == "" || request.URL.User != nil {
		return errors.New("remote plugin redirect must use HTTPS without userinfo")
	}
	return nil
}

func (s *InstallService) runInstall(job installJob) error {
	if job.inspection == nil {
		return installError(codeInvalidRequest, "插件安装缺少有效检查结果", "插件安装缺少有效检查结果")
	}
	workingRoot := job.inspection.workingRoot
	candidateDir := job.inspection.candidateDir

	if err := job.ctx.Err(); err != nil {
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(20),
		Summary:  stringPtr("校验插件 manifest"),
	})

	candidateSnapshot := job.inspection.snapshot
	metadata := job.inspection.metadata
	existingSnapshot, exists := s.catalog.Get(candidateSnapshot.PluginID)
	if exists && !job.request.ReplaceExisting {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	}
	if exists && existingSnapshot.SourceRoot != "plugins/installed" {
		return installError(codePluginInstallFailed, "同 ID 插件不属于统一安装目录", "同 ID 插件无法原子替换")
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
		Summary:  stringPtr("复核插件 artifact"),
	})

	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		return installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
	}
	if _, err := artifact.Verify(candidateDir, artifact.Options{ExpectedPlatform: targetPlatform}); err != nil {
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
		}
		return installError(codePluginArtifactInvalid, err.Error(), "插件 artifact 校验失败")
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(60),
		Summary:  stringPtr("写入正式安装目录"),
	})

	if err := os.MkdirAll(s.installedRoot, 0o755); err != nil {
		return installError(codePluginInstallFailed, "创建插件安装目录失败", "创建插件安装目录失败")
	}

	finalTarget := filepath.Join(s.installedRoot, candidateSnapshot.PluginID)
	_, finalErr := s.deps.stat(finalTarget)
	if finalErr == nil && !job.request.ReplaceExisting {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	} else if finalErr != nil && !errors.Is(finalErr, os.ErrNotExist) {
		return installError(codePluginInstallFailed, "检查插件安装目录失败", "检查插件安装目录失败")
	}

	previousTarget := filepath.Join(workingRoot, "previous")
	replacing := finalErr == nil
	var previousMetadata plugins.PackageMetadata
	hadPreviousMetadata := false
	if replacing && s.packageRepo != nil {
		loader, ok := s.repository.(plugins.PackageMetadataLoader)
		if !ok {
			return installError(codePluginInstallFailed, "安装元数据仓库不支持更新回滚", "插件更新未写入")
		}
		all, loadErr := loader.LoadAllPackageMetadata(job.ctx)
		if loadErr != nil {
			return installError(codePluginInstallFailed, "读取当前插件安装元数据失败", "插件更新未写入")
		}
		previousMetadata, hadPreviousMetadata = all[candidateSnapshot.PluginID]
	}
	resumePrevious := func() {
		if s.afterRollback != nil {
			s.afterRollback(context.WithoutCancel(job.ctx), candidateSnapshot.PluginID)
		}
	}
	if replacing {
		if s.beforeReplace != nil {
			s.beforeReplace(job.ctx, candidateSnapshot.PluginID)
		}
		if err := s.deps.rename(finalTarget, previousTarget); err != nil {
			resumePrevious()
			return installError(codePluginInstallFailed, "备份当前插件版本失败", "插件更新未写入")
		}
	}

	if err := s.deps.rename(candidateDir, finalTarget); err != nil {
		if replacing {
			_ = s.deps.rename(previousTarget, finalTarget)
			resumePrevious()
		}
		return installError(codePluginInstallFailed, "写入插件安装目录失败", "写入插件安装目录失败")
	}

	rollback := func() {
		cleanupCtx := context.WithoutCancel(job.ctx)
		_ = s.deps.removeAll(finalTarget)
		if replacing {
			_ = s.deps.rename(previousTarget, finalTarget)
		}
		if s.packageRepo != nil {
			if hadPreviousMetadata {
				_ = s.packageRepo.SavePackageMetadata(cleanupCtx, previousMetadata)
			} else {
				_ = s.packageRepo.DeletePackageMetadata(cleanupCtx, candidateSnapshot.PluginID)
			}
		}
		_ = s.refreshCatalog(cleanupCtx)
		resumePrevious()
	}

	if s.packageRepo != nil {
		metadata.InstalledAt = s.deps.now().UTC()
		if err := s.packageRepo.SavePackageMetadata(job.ctx, metadata); err != nil {
			rollback()
			return installError(codePluginInstallFailed, "写入插件安装元数据失败", "写入插件安装元数据失败")
		}
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(75),
		Summary:  stringPtr("刷新插件目录索引"),
	})

	if err := s.refreshCatalog(job.ctx); err != nil {
		rollback()
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(90),
		Summary:  stringPtr("写入安装元数据"),
	})

	if s.afterSuccess != nil {
		if err := s.afterSuccess(job.ctx, candidateSnapshot.PluginID); err != nil {
			rollback()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return installError(codePluginInstallFailed, err.Error(), "插件安装后处理失败")
		}
	}

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
		PluginID:          snapshot.PluginID,
		SourceType:        request.SourceType,
		SourceRef:         request.Source,
		Version:           snapshot.Version,
		ManifestHash:      manifestHash,
		PackageHash:       packageHash,
		ArchiveHash:       request.ExpectedArchiveSHA256,
		PublisherID:       request.PublisherID,
		PublisherName:     request.PublisherName,
		PublisherVerified: request.PublisherVerified,
		CatalogDigest:     request.CatalogDigest,
	}, nil
}

func newInspectionID() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func installSourceLabel(request plugins.InstallRequest) string {
	if request.SourceType == "catalog" {
		return request.PublisherName
	}
	if request.SourceType == "development" {
		return "development workspace"
	}
	if request.SourceType == "remote_url" {
		if parsed, err := url.Parse(request.Source); err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	label := filepath.Base(filepath.Clean(request.Source))
	if label == "." || label == string(filepath.Separator) || label == "" {
		return request.SourceType
	}
	return label
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

	sourceType := request.SourceType
	source := request.Source
	if request.ResolvedSourceType != "" {
		sourceType = request.ResolvedSourceType
		source = request.ResolvedSource
	}
	switch sourceType {
	case "local_directory":
		info, err := s.deps.stat(source)
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
		if err := s.deps.copyDir(ctx, source, candidate); err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, installError(codePluginInstallFailed, "复制插件来源目录失败", "复制插件来源目录失败")
		}
		return tempRoot, candidate, cleanup, nil
	case "local_zip":
		info, err := s.deps.stat(source)
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
		if request.ExpectedArchiveSize > 0 && info.Size() != request.ExpectedArchiveSize {
			cleanup()
			return "", "", func() {}, installError("plugin.store_integrity_mismatch", "插件压缩包大小与商店目录不一致", "插件商店产物完整性校验失败")
		}

		if request.ExpectedArchiveSHA256 != "" {
			digest, hashErr := s.deps.hashFile(source)
			if hashErr != nil || digest != request.ExpectedArchiveSHA256 {
				cleanup()
				return "", "", func() {}, installError("plugin.store_integrity_mismatch", "插件压缩包摘要与商店目录不一致", "插件商店产物完整性校验失败")
			}
		}
		candidate, err := s.deps.extractZip(ctx, source, tempRoot)
		if err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, err
		}
		return tempRoot, candidate, cleanup, nil
	case "remote_url":
		parsed, err := url.Parse(source)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			cleanup()
			return "", "", func() {}, installError(codeInvalidRequest, "远程来源必须是 HTTPS URL", "远程来源必须是 HTTPS URL")
		}

		downloadPath := filepath.Join(tempRoot, "download.zip")
		if err := s.deps.downloadFile(ctx, source, downloadPath); err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			if errors.Is(err, errPluginPackageResourceLimit) {
				return "", "", func() {}, installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
			}
			return "", "", func() {}, installError(codePluginInstallFailed, "下载远程插件压缩包失败", "下载远程插件压缩包失败")
		}
		if request.ExpectedArchiveSize > 0 {
			info, statErr := s.deps.stat(downloadPath)
			if statErr != nil || info.Size() != request.ExpectedArchiveSize {
				cleanup()
				return "", "", func() {}, installError("plugin.store_integrity_mismatch", "下载的插件大小与商店目录不一致", "插件商店产物完整性校验失败")
			}
		}

		if request.ExpectedArchiveSHA256 != "" {
			digest, hashErr := s.deps.hashFile(downloadPath)
			if hashErr != nil || digest != request.ExpectedArchiveSHA256 {
				cleanup()
				return "", "", func() {}, installError("plugin.store_integrity_mismatch", "下载的插件摘要与商店目录不一致", "插件商店产物完整性校验失败")
			}
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
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return "", installError(codePluginInstallFailed, "读取插件压缩包失败", "读取插件压缩包失败")
	}
	if archiveInfo.Size() > maxRemoteDownloadBytes {
		return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", installError(codePluginInstallFailed, "解压插件压缩包失败", "解压插件压缩包失败")
	}
	defer reader.Close()
	if len(reader.File) > maxPluginArchiveEntries {
		return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
	}

	extractRoot := filepath.Join(tempRoot, "unzipped")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", installError(codePluginInstallFailed, "创建解压临时目录失败", "创建解压临时目录失败")
	}

	topLevels := map[string]struct{}{}
	seenPaths := make(map[string]struct{}, len(reader.File))
	var expandedBytes uint64

	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		cleanName := filepath.Clean(file.Name)
		if cleanName == "." || filepath.IsAbs(cleanName) || filepath.VolumeName(cleanName) != "" || strings.Contains(cleanName, ":") || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
			return "", installError(codePackageUnsafeEntry, "插件包包含不安全文件", "插件包包含不安全文件")
		}
		canonicalName := strings.ToLower(filepath.ToSlash(cleanName))
		if _, exists := seenPaths[canonicalName]; exists {
			return "", installError(codePackageUnsafeEntry, "插件包包含不安全文件", "插件包包含不安全文件")
		}
		seenPaths[canonicalName] = struct{}{}

		mode := file.Mode()
		if mode&(os.ModeSymlink|os.ModeDevice|os.ModeCharDevice|os.ModeNamedPipe|os.ModeSocket|os.ModeIrregular) != 0 {
			return "", installError(codePackageUnsafeEntry, "插件包包含不安全文件", "插件包包含不安全文件")
		}
		if !file.FileInfo().IsDir() {
			if file.UncompressedSize64 > maxPluginArchiveFileBytes {
				return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
			}
			if exceedsCompressionRatio(file.UncompressedSize64, file.CompressedSize64, maxPluginArchiveRatio) {
				return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
			}
			if expandedBytes > maxPluginArchiveExpandBytes-file.UncompressedSize64 {
				return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
			}
			expandedBytes += file.UncompressedSize64
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

		written, copyErr := io.Copy(targetFile, io.LimitReader(readerHandle, maxPluginArchiveFileBytes+1))
		if copyErr != nil {
			targetFile.Close()
			readerHandle.Close()
			return "", installError(codePluginInstallFailed, "写入解压文件失败", "写入解压文件失败")
		}
		if written > maxPluginArchiveFileBytes || uint64(written) != file.UncompressedSize64 {
			targetFile.Close()
			readerHandle.Close()
			return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
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

func exceedsCompressionRatio(uncompressed, compressed uint64, maxRatio uint64) bool {
	if uncompressed == 0 {
		return false
	}
	if compressed == 0 || maxRatio == 0 {
		return true
	}
	quotient := uncompressed / compressed
	return quotient > maxRatio || quotient == maxRatio && uncompressed%compressed != 0
}
