package spec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func runtimeCommand(ctx context.Context, runtimeName string, repoRoot string, manifestDir string, runtimeConfig config.RuntimeConfig) (string, []string, error) {
	manager := deps.NewManager(repoRoot)
	switch runtimeName {
	case "python":
		if venvPython, ok := pythonVirtualenvExecutable(manifestDir); ok {
			return venvPython, pythonRuntimeEnvironment(), nil
		}
		command, err := manager.ResolveEntrypoint(ctx, "python-runtime", "python")
		if err != nil {
			return "", nil, errorf(codePlatformResourceMissing, "managed Python runtime is not available", err)
		}
		return command, pythonRuntimeEnvironment(), nil
	case "nodejs":
		command, err := manager.ResolveEntrypoint(ctx, "nodejs-runtime", "node")
		if err != nil {
			return "", nil, errorf(codePlatformResourceMissing, "managed Node.js runtime is not available", err)
		}
		env := nodeRuntimeEnvironment(runtimeConfig)
		return command, env, nil
	default:
		return "", nil, errorf(codePlatformInvalidRequest, "plugin runtime is not supported by the minimal runtime manager", nil)
	}
}

func pythonRuntimeEnvironment() []string {
	return []string{
		"PYTHONIOENCODING=UTF-8",
		"PYTHONUTF8=1",
		"PYTHONUNBUFFERED=1",
	}
}

func pythonVirtualenvExecutable(manifestDir string) (string, bool) {
	candidates := []string{
		filepath.Join(manifestDir, ".venv", "bin", "python"),
		filepath.Join(manifestDir, ".venv", "bin", "python3"),
		filepath.Join(manifestDir, ".venv", "Scripts", "python.exe"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func nodeRuntimeEnvironment(runtimeConfig config.RuntimeConfig) []string {
	if runtimeConfig.NodeMaxOldSpaceSizeMB <= 0 {
		return nil
	}
	return []string{fmt.Sprintf("NODE_OPTIONS=--max-old-space-size=%d", runtimeConfig.NodeMaxOldSpaceSizeMB)}
}

func durationFromSeconds(seconds int, fallback int) time.Duration {
	if seconds <= 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}
