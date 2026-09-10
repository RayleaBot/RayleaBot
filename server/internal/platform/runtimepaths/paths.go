package runtimepaths

import (
	"fmt"
	"path/filepath"
)

func ResolveDatabasePath(configPath, databasePath string) (string, error) {
	if filepath.IsAbs(databasePath) {
		return filepath.Clean(databasePath), nil
	}

	repoRoot, err := ResolveRuntimeRoot(configPath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(filepath.Join(repoRoot, databasePath))
	if err != nil {
		return "", fmt.Errorf("resolve database path %s: %w", databasePath, err)
	}

	return resolved, nil
}

func ResolveRuntimeRoot(configPath string) (string, error) {
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve runtime root from %s: %w", configPath, err)
	}
	return RootFromConfigPath(absoluteConfigPath), nil
}

func ResolveConfigLifecycleLockPath(configPath string) (string, error) {
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve config lifecycle lock from %s: %w", configPath, err)
	}
	return filepath.Clean(absoluteConfigPath) + ".runtime.lock", nil
}

func RootFromConfigPath(configPath string) string {
	return filepath.Dir(filepath.Dir(configPath))
}
