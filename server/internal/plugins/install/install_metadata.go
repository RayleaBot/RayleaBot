package install

import (
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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
