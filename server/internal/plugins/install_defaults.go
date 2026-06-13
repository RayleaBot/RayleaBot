package plugins

import (
	"context"
	"errors"
	"os"
	"time"
)

func installedDiscoveryRoot(discoveryRoots []ScanRoot) (string, error) {
	for _, root := range discoveryRoots {
		if root.Label == "plugins/installed" {
			return root.Path, nil
		}
	}
	return "", errors.New("plugins/installed discovery root is required")
}

func withDefaultInstallerDeps(repoRoot string, deps installerDeps) installerDeps {
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.copyDir == nil {
		deps.copyDir = copyDirectory
	}
	if deps.extractZip == nil {
		deps.extractZip = extractZipSource
	}
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.rename == nil {
		deps.rename = os.Rename
	}
	if deps.stat == nil {
		deps.stat = os.Stat
	}
	if deps.readDir == nil {
		deps.readDir = os.ReadDir
	}
	if deps.hashFile == nil {
		deps.hashFile = hashFileSHA256
	}
	if deps.hashDir == nil {
		deps.hashDir = hashDirectorySHA256
	}
	if deps.preparePython == nil {
		deps.preparePython = func(ctx context.Context, pluginDir string, dependencies []string) error {
			return preparePythonEnvironment(ctx, repoRoot, pluginDir, dependencies)
		}
	}
	if deps.prepareNode == nil {
		deps.prepareNode = func(ctx context.Context, pluginDir string, dependencies []string, allowInstallScripts bool) error {
			return prepareNodeEnvironment(ctx, repoRoot, pluginDir, dependencies, allowInstallScripts)
		}
	}
	if deps.downloadFile == nil {
		deps.downloadFile = downloadHTTPSFile
	}
	return deps
}
