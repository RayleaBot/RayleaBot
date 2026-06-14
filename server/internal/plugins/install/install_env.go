package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

type runtimeResolver interface {
	ResolveEntrypoint(context.Context, string, string) (string, error)
}

var newRuntimeResolver = func(repoRoot string) runtimeResolver {
	return deps.NewManager(repoRoot)
}

var runManagedCommand = executeManagedCommand

func preparePythonEnvironment(ctx context.Context, repoRoot string, pluginDir string, dependencies []string) error {
	if len(dependencies) == 0 {
		return nil
	}

	resolver := newRuntimeResolver(repoRoot)
	pythonExecutable, err := resolver.ResolveEntrypoint(ctx, "python-runtime", "python")
	if err != nil {
		return err
	}

	venvDir := filepath.Join(pluginDir, ".venv")
	if err := runManagedCommand(ctx, pluginDir, nil, pythonExecutable, "-m", "venv", venvDir); err != nil {
		return err
	}

	venvPython, err := virtualenvPythonExecutable(venvDir)
	if err != nil {
		return err
	}
	args := append([]string{"-m", "pip", "install"}, dependencies...)
	return runManagedCommand(ctx, pluginDir, nil, venvPython, args...)
}

func prepareNodeEnvironment(ctx context.Context, repoRoot string, pluginDir string, dependencies []string, allowInstallScripts bool) error {
	packageJSONPath := filepath.Join(pluginDir, "package.json")
	_, err := os.Stat(packageJSONPath)
	hasPackageJSON := err == nil

	if len(dependencies) == 0 && !hasPackageJSON {
		return nil
	}

	resolver := newRuntimeResolver(repoRoot)
	npmExecutable, err := resolver.ResolveEntrypoint(ctx, "nodejs-runtime", "npm")
	if err != nil {
		return err
	}

	userConfigPath := filepath.Join(pluginDir, ".npmrc.managed")
	if err := os.WriteFile(userConfigPath, nil, 0o644); err != nil {
		return err
	}

	args := buildNodeInstallArgs(pluginDir, dependencies, allowInstallScripts, hasPackageJSON)
	env := []string{"NPM_CONFIG_USERCONFIG=" + userConfigPath}
	return runManagedCommand(ctx, pluginDir, env, npmExecutable, args...)
}

func buildNodeInstallArgs(pluginDir string, dependencies []string, allowInstallScripts bool, hasPackageJSON bool) []string {
	args := []string{}
	hasPackageLock := false
	if hasPackageJSON {
		for _, name := range []string{"package-lock.json", "npm-shrinkwrap.json"} {
			if _, err := os.Stat(filepath.Join(pluginDir, name)); err == nil {
				hasPackageLock = true
				break
			}
		}
	}
	if hasPackageLock {
		args = append(args, "ci")
	} else {
		args = append(args, "install")
	}
	if !allowInstallScripts {
		args = append(args, "--ignore-scripts")
	}
	if !hasPackageJSON {
		args = append(args, dependencies...)
	}
	return args
}

func virtualenvPythonExecutable(venvDir string) (string, error) {
	candidates := []string{
		filepath.Join(venvDir, "bin", "python"),
		filepath.Join(venvDir, "bin", "python3"),
		filepath.Join(venvDir, "Scripts", "python.exe"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("virtualenv python executable is missing under %s", venvDir)
}
