package plugins

import "context"

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
