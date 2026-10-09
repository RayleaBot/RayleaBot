package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
	semverutil "github.com/RayleaBot/RayleaBot/server/internal/platform/semver"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

// installCandidate is a verified package staged under the installed root,
// waiting for its install task.
type installCandidate struct {
	request      plugins.InstallRequest
	workingRoot  string
	candidateDir string
	cleanup      func()
	snapshot     plugins.Snapshot
	metadata     plugins.PackageMetadata
}

// Accept verifies the package in the request and queues its installation.
func (s *InstallService) Accept(ctx context.Context, request plugins.InstallRequest) (string, error) {
	if request.TrustedCodeRequired && !request.TrustedCodeConfirmed {
		return "", plugins.ErrTrustedCodeConfirmation
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return "", context.Canceled
	}
	candidate, err := s.prepareCandidate(ctx, request)
	if err != nil {
		return "", err
	}
	return s.enqueue(candidate)
}

func (s *InstallService) enqueue(candidate *installCandidate) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		candidate.cleanup()
		return "", context.Canceled
	}
	if !s.admission.TryAcquire() {
		candidate.cleanup()
		return "", tasks.ErrQueueFull
	}
	taskID, err := s.registry.Create("plugin.install", "安装插件“"+installPluginName(candidate.snapshot)+"”")
	if err != nil {
		s.admission.Release()
		candidate.cleanup()
		return "", err
	}
	runCtx, cancel := context.WithTimeout(s.baseCtx, s.timeout)
	s.cancels[taskID] = cancel
	s.jobs <- installJob{taskID: taskID, request: candidate.request, candidate: candidate, ctx: runCtx}
	return taskID, nil
}

func (s *InstallService) prepareCandidate(ctx context.Context, request plugins.InstallRequest) (*installCandidate, error) {
	workingRoot, candidateDir, cleanup, err := s.prepareSource(ctx, request)
	if err != nil {
		return nil, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			cleanup()
		}
	}()

	// New manifest fields may require a newer core; check that requirement
	// before either artifact verification or snapshot loading validates the manifest.
	if minVersion := readMinimumCoreVersion(candidateDir); minVersion != "" {
		if err := plugins.CheckCoreVersion(releaseupdate.InstalledVersion(s.repoRoot), minVersion); err != nil {
			if err.Reason != plugins.CoreVersionUnknown || request.SourceType != "development" {
				return nil, err
			}
		}
	}
	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		return nil, installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
	}
	verified, err := artifact.Verify(candidateDir, artifact.Options{ExpectedPlatform: targetPlatform})
	if err != nil {
		if errors.Is(err, artifact.ErrContractUnsupported) {
			return nil, installError(errorcodes.PluginContractUnsupported, err.Error(), "插件合同版本不受支持")
		}
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return nil, installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
		}
		return nil, installError(codePluginArtifactInvalid, err.Error(), "插件 artifact 校验失败")
	}
	if err := markBackendExecutable(verified.BackendPath); err != nil {
		return nil, installError(codePluginInstallFailed, "设置插件入口可执行权限失败", "设置插件入口可执行权限失败")
	}
	snapshot, err := s.loadCandidateSnapshot(candidateDir)
	if err != nil {
		return nil, err
	}
	if request.ExpectedPluginID != "" && (snapshot.PluginID != request.ExpectedPluginID || snapshot.Version != request.ExpectedVersion) {
		return nil, installError(errorcodes.PluginStoreIntegrityMismatch, "插件包与商店条目不一致", "插件包与商店条目不一致")
	}
	if request.SourceType != "development" {
		if err := plugins.CheckRequiredDependencies(snapshot.Dependencies, s.catalog.List()); err != nil {
			return nil, err
		}
	}
	metadata, err := s.buildPackageMetadata(ctx, request, snapshot, candidateDir)
	if err != nil {
		return nil, err
	}
	succeeded = true
	return &installCandidate{request: request, workingRoot: workingRoot, candidateDir: candidateDir, cleanup: cleanup, snapshot: snapshot, metadata: metadata}, nil
}

func readMinimumCoreVersion(candidateDir string) string {
	payload, err := os.ReadFile(filepath.Join(candidateDir, "info.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		MinCoreVersion string `json:"min_core_version"`
	}
	if json.Unmarshal(payload, &manifest) != nil || !semverutil.Valid(manifest.MinCoreVersion) {
		return ""
	}
	return manifest.MinCoreVersion
}

// markBackendExecutable sets the entry's executable bits on Unix, because ZIP
// archives and copied directories do not reliably preserve them.
func markBackendExecutable(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0o755)
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

func (s *InstallService) buildPackageMetadata(ctx context.Context, request plugins.InstallRequest, snapshot plugins.Snapshot, candidateDir string) (plugins.PackageMetadata, error) {
	packageHash, err := fsguard.SHA256Directory(ctx, candidateDir)
	if err != nil {
		return plugins.PackageMetadata{}, installError(codePluginInstallFailed, "计算插件安装包哈希失败", "计算插件安装包哈希失败")
	}

	return plugins.PackageMetadata{
		PluginID:    snapshot.PluginID,
		SourceType:  request.SourceType,
		SourceRef:   request.Source,
		Version:     snapshot.Version,
		PackageHash: packageHash,
	}, nil
}
