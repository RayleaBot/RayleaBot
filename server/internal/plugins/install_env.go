package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func hashDirectorySHA256(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", root)
	}

	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, relativePath)
		return nil
	}); err != nil {
		return "", err
	}

	sort.Strings(files)
	hasher := sha256.New()
	for _, relativePath := range files {
		if _, err := io.WriteString(hasher, filepath.ToSlash(relativePath)); err != nil {
			return "", err
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return "", err
		}

		file, err := os.Open(filepath.Join(root, relativePath))
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hasher, file); err != nil {
			file.Close()
			return "", err
		}
		file.Close()
		if _, err := hasher.Write([]byte{0}); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

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

func executeManagedCommand(ctx context.Context, dir string, env []string, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = append([]string(nil), os.Environ()...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if len(output) != 0 {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return execErr.Err
	}
	return err
}
