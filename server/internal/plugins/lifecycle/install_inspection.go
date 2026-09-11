package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
	semverutil "github.com/RayleaBot/RayleaBot/server/internal/platform/semver"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type installInspectionEntry struct {
	inspection   plugins.InstallInspection
	request      plugins.InstallRequest
	workingRoot  string
	candidateDir string
	cleanup      func()
	snapshot     plugins.Snapshot
	metadata     plugins.PackageMetadata
}

func (s *InstallService) Accept(_ context.Context, acceptance plugins.InstallAcceptance) (string, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", context.Canceled
	}
	entry, err := s.consumeInspectionLocked(acceptance)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	if entry.request.TrustedCodeRequired && !acceptance.TrustedCodeConfirmed {
		s.mu.Unlock()
		return "", plugins.ErrTrustedCodeConfirmation
	}
	if !s.admission.TryAcquire() {
		s.mu.Unlock()
		return "", tasks.ErrQueueFull
	}

	taskID, err := s.registry.Create("plugin.install", "安装插件“"+installPluginName(entry.snapshot)+"”")
	if err != nil {
		s.admission.Release()
		s.mu.Unlock()
		return "", err
	}

	runCtx, cancel := context.WithTimeout(s.baseCtx, s.timeout)
	s.cancels[taskID] = cancel
	delete(s.inspections, acceptance.InspectionID)
	s.jobs <- installJob{taskID: taskID, request: entry.request, inspection: entry, ctx: runCtx}
	s.mu.Unlock()
	return taskID, nil
}

func (s *InstallService) Inspect(ctx context.Context, request plugins.InstallRequest) (plugins.InstallInspection, error) {
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
		if errors.Is(err, artifact.ErrContractUnsupported) {
			return plugins.InstallInspection{}, installError(errorcodes.PluginContractUnsupported, err.Error(), "插件合同版本不受支持")
		}
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return plugins.InstallInspection{}, installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
		}
		return plugins.InstallInspection{}, installError(codePluginArtifactInvalid, err.Error(), "插件 artifact 校验失败")
	}
	snapshot, err := s.loadCandidateSnapshot(candidateDir)
	if err != nil {
		return plugins.InstallInspection{}, err
	}
	coreVersion := releaseupdate.InstalledVersion(s.repoRoot)
	unknownVersion := coreVersion == "unknown"
	if (unknownVersion && request.SourceType != "development") || (!unknownVersion && semverutil.Compare(coreVersion, snapshot.MinCoreVersion) < 0) {
		return plugins.InstallInspection{}, installError(
			errorcodes.PluginCoreVersionIncompatible,
			fmt.Sprintf("插件要求 RayleaBot %s 或更高版本，当前版本为 %s", snapshot.MinCoreVersion, coreVersion),
			"插件与当前 RayleaBot 版本不兼容",
		)
	}
	metadata, err := s.buildPackageMetadata(ctx, request, snapshot, candidateDir)
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
		Permissions:    plugins.ClonePermissions(snapshot.Permissions),
		TargetPlatform: verified.Document.TargetPlatform,
		Artifact: plugins.ArtifactInspection{
			Valid:     true,
			Version:   verified.Document.ArtifactVersion,
			FileCount: verified.FileCount,
		},
	}
	inspection.Backend = plugins.InstallBackendInspection{
		Entry: verified.Document.Entry,
		Path:  verified.Document.Entry,
		Size:  verified.BackendSize,
	}
	inspection.UI.FileCount = verified.UIFileCount
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

func (s *InstallService) consumeInspectionLocked(acceptance plugins.InstallAcceptance) (*installInspectionEntry, error) {
	id := strings.TrimSpace(acceptance.InspectionID)
	if id == "" || strings.TrimSpace(acceptance.PackageSHA256) == "" {
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
	if acceptance.PackageSHA256 != entry.inspection.PackageSHA256 {
		return nil, plugins.ErrInstallDigestMismatch
	}
	return entry, nil
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

func newInspectionID() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func installSourceLabel(request plugins.InstallRequest) string {
	if request.SourceType == "catalog" {
		if strings.TrimSpace(request.SourceLabel) != "" {
			return strings.TrimSpace(request.SourceLabel)
		}
		return request.Source
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
