package runtimepaths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultConfigPath = "config/user.yaml"

// ResolveStartupConfigPath chooses the configuration before any runtime files
// are opened. An explicitly supplied path selects its own runtime directory.
func ResolveStartupConfigPath(configPath string, explicit bool) (string, error) {
	if explicit {
		if configPath == "" {
			return "", errors.New("config path is required")
		}
		return filepath.Abs(configPath)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve server executable: %w", err)
	}
	return defaultConfigPath(workingDirectory, executable), nil
}

func defaultConfigPath(workingDirectory, executable string) string {
	for _, start := range []string{filepath.Dir(executable), workingDirectory} {
		if root := findInstallationRoot(start); root != "" {
			return filepath.Join(root, filepath.FromSlash(DefaultConfigPath))
		}
	}
	return filepath.Join(workingDirectory, filepath.FromSlash(DefaultConfigPath))
}

func findInstallationRoot(start string) string {
	for current := filepath.Clean(start); ; current = filepath.Dir(current) {
		source := regularFile(filepath.Join(current, "server", "go.mod")) && regularFile(filepath.Join(current, "launcher", "package.json"))
		release := regularFile(filepath.Join(current, "build_info.json")) && regularFile(filepath.Join(current, ".deps", "manifest.json"))
		if source || release {
			return current
		}
		if filepath.Dir(current) == current {
			return ""
		}
	}
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
