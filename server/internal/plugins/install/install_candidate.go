package install

import (
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginmanifest "github.com/RayleaBot/RayleaBot/server/internal/plugins/manifest"
)

func (s *InstallService) loadCandidateSnapshot(candidateDir string) (plugins.Snapshot, error) {
	infoPath := filepath.Join(candidateDir, "info.json")
	snapshot, ok, err := pluginmanifest.LoadSnapshot(infoPath, "plugins/installed", s.repoRoot, s.validator, pluginmanifest.ManifestValidationMaxSummary, s.logger)
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
