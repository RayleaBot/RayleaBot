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
	runtimepkg "runtime"
	"sort"
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

func preparePythonEnvironment(ctx context.Context, pluginDir string, dependencies []string) error {
	if len(dependencies) == 0 {
		return nil
	}

	venvDir := filepath.Join(pluginDir, ".venv")
	if err := runCommand(ctx, pluginDir, pythonExecutableCandidates(), "-m", "venv", venvDir); err != nil {
		return err
	}

	pythonExecutable := filepath.Join(venvDir, "bin", "python")
	if runtimepkg.GOOS == "windows" {
		pythonExecutable = filepath.Join(venvDir, "Scripts", "python.exe")
	}

	args := append([]string{"-m", "pip", "install"}, dependencies...)
	return runCommand(ctx, pluginDir, []string{pythonExecutable}, args...)
}

func prepareNodeEnvironment(ctx context.Context, pluginDir string, dependencies []string, allowInstallScripts bool) error {
	packageJSONPath := filepath.Join(pluginDir, "package.json")
	_, err := os.Stat(packageJSONPath)
	hasPackageJSON := err == nil

	if len(dependencies) == 0 && !hasPackageJSON {
		return nil
	}

	args := []string{"install", "--no-package-lock", "--omit=dev"}
	if !allowInstallScripts {
		args = append(args, "--ignore-scripts")
	}
	if !hasPackageJSON {
		args = append(args, dependencies...)
	}

	return runCommand(ctx, pluginDir, []string{npmExecutable()}, args...)
}

func runCommand(ctx context.Context, dir string, names []string, args ...string) error {
	var lastErr error
	for _, name := range names {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = err
		if len(output) != 0 {
			lastErr = fmt.Errorf("%w: %s", err, string(output))
		}
		if isExecutableNotFound(err) {
			continue
		}
		return lastErr
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no executable candidates provided")
	}
	return lastErr
}

func pythonExecutableCandidates() []string {
	if runtimepkg.GOOS == "windows" {
		return []string{"python"}
	}
	return []string{"python3", "python"}
}

func npmExecutable() string {
	if runtimepkg.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func isExecutableNotFound(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return execErr.Err == exec.ErrNotFound
	}
	return false
}
