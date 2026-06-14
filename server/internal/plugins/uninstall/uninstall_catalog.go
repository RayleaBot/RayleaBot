package uninstall

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
)

func (s *UninstallService) refreshCatalog() error {
	snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: s.validator,
		Roots:     s.discoveryRoots,
		RepoRoot:  s.repoRoot,
		Logger:    s.logger,
	})
	if err != nil {
		return err
	}

	if packageLoader, ok := s.repository.(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			return err
		}
		snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(context.Background())
		if err != nil {
			return err
		}
		snapshots = plugins.ApplyDesiredStates(snapshots, states)
	}

	s.catalog.Replace(snapshots)
	return nil
}
