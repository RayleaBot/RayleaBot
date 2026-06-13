package plugins

import "context"

func (s *UninstallService) refreshCatalog() error {
	snapshots, _, err := Discover(DiscoverOptions{
		Validator: s.validator,
		Roots:     s.discoveryRoots,
		RepoRoot:  s.repoRoot,
		Logger:    s.logger,
	})
	if err != nil {
		return err
	}

	reloaded := NewCatalog(snapshots)
	if packageLoader, ok := s.repository.(PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			return err
		}
		reloaded.Replace(ApplyPackageMetadata(reloaded.List(), packageMetadata))
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(context.Background())
		if err != nil {
			return err
		}
		reloaded.ApplyDesiredStates(states)
	}

	s.catalog.Replace(reloaded.List())
	return nil
}
