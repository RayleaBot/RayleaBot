package cli

import (
	"fmt"
	"path/filepath"
)

func resolveDatabasePath(configPath string) (string, error) {
	configDir := filepath.Dir(configPath)
	dbPath := filepath.Join(configDir, "..", "data", "rayleabot.db")
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	return absPath, nil
}
