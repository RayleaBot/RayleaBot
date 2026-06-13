package plugins

import "path/filepath"

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
