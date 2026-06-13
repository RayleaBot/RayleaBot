package plugininstall

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
)

func (s *InstallService) refreshCatalog() error {
	snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: s.validator,
		Roots:     s.discoveryRoots,
		RepoRoot:  s.repoRoot,
		Logger:    s.logger,
	})
	if err != nil {
		return installError(codePluginInstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败")
	}

	if packageLoader, ok := s.repository.(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件安装元数据失败", "读取插件安装元数据失败")
		}
		snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(context.Background())
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件持久化状态失败", "读取插件持久化状态失败")
		}
		snapshots = plugins.ApplyDesiredStates(snapshots, states)
	}

	s.catalog.Replace(snapshots)
	return nil
}
