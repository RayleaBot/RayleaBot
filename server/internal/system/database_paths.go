package system

import (
	"fmt"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func databasePathResolver(resolver DatabasePathResolver) DatabasePathResolver {
	if resolver != nil {
		return resolver
	}
	return defaultDatabasePath
}

func (s *Service) databasePath(configPath, configuredPath string) (string, error) {
	if s != nil && s.resolveDatabasePath != nil {
		return s.resolveDatabasePath(configPath, configuredPath)
	}
	return defaultDatabasePath(configPath, configuredPath)
}

func defaultDatabasePath(configPath, configuredPath string) (string, error) {
	if filepath.IsAbs(configuredPath) {
		return filepath.Clean(configuredPath), nil
	}

	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve runtime root from %s: %w", configPath, err)
	}
	repoRoot := recovery.RepoRootFromConfigPath(absoluteConfigPath)
	resolved, err := filepath.Abs(filepath.Join(repoRoot, configuredPath))
	if err != nil {
		return "", fmt.Errorf("resolve database path %s: %w", configuredPath, err)
	}
	return resolved, nil
}
