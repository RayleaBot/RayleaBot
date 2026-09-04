package lifecycle

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
)

// StartDevelopment waits for initialization before the installer commits a replacement.
// The installer has already stopped the previous runtime and refreshed the catalog.
func (c *Controller) StartDevelopment(ctx context.Context, pluginID string) error {
	ctx, cancel := context.WithTimeout(ctx, runtimeInitTimeout(c.config().Runtime))
	defer cancel()
	snapshot, exists := c.plugins.Get(pluginID)
	if !exists {
		return plugins.ErrPluginNotFound
	}
	settings := pluginstore.MergeValues(snapshot.DefaultConfig, nil)
	if c.pluginConfig != nil {
		persisted, err := c.pluginConfig.ReadAll(ctx, pluginID)
		if err != nil {
			return err
		}
		settings = pluginstore.MergeValues(snapshot.DefaultConfig, persisted)
	}
	c.plugins.RefreshCommands(pluginID, settings)
	manager := c.runtimes.GetOrCreate(pluginID)
	if err := c.startRuntime(ctx, pluginID, c.currentBotID(), manager); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		c.StopAndResetPluginWithContext(cleanupCtx, pluginID)
		return err
	}
	return nil
}

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
		candidateHash, hashErr := s.deps.hashDir(artifactPath)
		if hashErr != nil {
			return "", false, hashErr
		}
		installedHash, hashErr := s.deps.hashDir(current.PackageRootPath)
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
