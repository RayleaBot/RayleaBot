package lifecycle

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

// SyncDevelopment admits a standard artifact through the existing installation queue.
// Unchanged packages do not create a task or alter the plugin's desired/runtime state.
func (s *InstallService) SyncDevelopment(ctx context.Context, artifactPath, sourcePath string) (string, bool, error) {
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		return "", false, err
	}
	if _, err := artifact.Verify(artifactPath, artifact.Options{ExpectedPlatform: platform}); err != nil {
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return "", false, installError(codePluginPlatformMismatch, "开发插件与当前平台不匹配", "开发插件与当前平台不匹配")
		}
		return "", false, installError(codePluginArtifactInvalid, "开发插件 artifact 校验失败", "开发插件 artifact 校验失败")
	}
	snapshot, err := s.loadCandidateSnapshot(artifactPath)
	if err != nil {
		return "", false, err
	}
	if current, exists := s.catalog.Get(snapshot.PluginID); exists && current.PackageSourceType == "development" && filepath.Clean(current.PackageSourceRef) == filepath.Clean(sourcePath) {
		candidateHash, hashErr := fsguard.SHA256Directory(ctx, artifactPath)
		if hashErr != nil {
			return "", false, hashErr
		}
		installedHash, hashErr := fsguard.SHA256Directory(ctx, current.PackageRootPath)
		if hashErr == nil && installedHash == candidateHash {
			return "", false, nil
		}
	}
	inspection, err := s.Inspect(ctx, plugins.InstallRequest{
		SourceType: "development", Source: sourcePath,
		ResolvedSourceType: "local_directory", ResolvedSource: artifactPath,
		ReplaceExisting: true,
	})
	if err != nil {
		return "", false, err
	}
	taskID, err := s.Accept(ctx, plugins.InstallAcceptance{
		InspectionID: inspection.InspectionID, PackageSHA256: inspection.PackageSHA256,
		TrustedCodeConfirmed: true,
	})
	return taskID, err == nil, err
}
