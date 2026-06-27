package spec

import (
	"context"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func BuildSpec(snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	return BuildSpecWithContext(context.Background(), snapshot, repoRoot, runtimeConfig)
}

func BuildSpecWithContext(ctx context.Context, snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	if snapshot.PluginID == "" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin_id is required for runtime startup", nil)
	}
	if !snapshot.Valid {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is not eligible for runtime startup", nil)
	}
	if snapshot.DisplayState == "conflict" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is conflicted and cannot be started", nil)
	}
	if snapshot.Runtime == "" || snapshot.Entry == "" || snapshot.ManifestPath == "" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is missing runtime startup fields", nil)
	}

	manifestPath := resolveManifestPath(repoRoot, snapshot.ManifestPath)
	manifestDir := filepath.Dir(manifestPath)

	if filepath.IsAbs(snapshot.Entry) {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must be relative to the manifest directory", nil)
	}

	entryPath := filepath.Clean(filepath.Join(manifestDir, filepath.FromSlash(snapshot.Entry)))
	if escapesDir(manifestDir, entryPath) {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must remain inside the plugin directory", nil)
	}
	entryInfo, err := os.Lstat(entryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin entry file is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "stat plugin entry file", err)
	}
	if entryInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(entryPath)
		if err != nil {
			return Spec{}, errorf(codePlatformResourceMissing, "resolve plugin entry symlink", err)
		}
		if escapesDir(manifestDir, resolveSymlinkTarget(entryPath, linkTarget)) {
			return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must remain inside the plugin directory", nil)
		}
	}

	resolvedManifestDir, err := filepath.EvalSymlinks(manifestDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin manifest directory is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "resolve plugin manifest directory", err)
	}

	resolvedEntryPath, err := filepath.EvalSymlinks(entryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin entry file is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "resolve plugin entry file", err)
	}
	if escapesDir(resolvedManifestDir, resolvedEntryPath) {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must remain inside the plugin directory", nil)
	}

	entryInfo, err = os.Stat(resolvedEntryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin entry file is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "stat plugin entry file", err)
	}
	if entryInfo.IsDir() {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must be a file", nil)
	}

	command, env, err := runtimeCommand(ctx, snapshot.Runtime, repoRoot, manifestDir, runtimeConfig)
	if err != nil {
		if runtimeErr, ok := err.(*Error); ok {
			return Spec{}, runtimeErr
		}
		return Spec{}, errorf(codePlatformResourceMissing, "resolve managed runtime executable", err)
	}

	initTimeout := durationFromSeconds(runtimeConfig.PluginInitTimeoutSeconds, 10)
	initMaxTotal := durationFromSeconds(runtimeConfig.PluginInitMaxTotalSeconds, 300)

	return Spec{
		PluginID:             snapshot.PluginID,
		PluginName:           snapshot.Name,
		RepoRoot:             repoRoot,
		Runtime:              snapshot.Runtime,
		Command:              command,
		Args:                 []string{resolvedEntryPath},
		Env:                  env,
		WorkDir:              resolvedManifestDir,
		EntryPath:            resolvedEntryPath,
		InitTimeout:          initTimeout,
		InitMaxTotal:         initMaxTotal,
		EventTimeout:         durationFromSeconds(runtimeConfig.PluginEventTimeoutSeconds, 5),
		ShutdownGrace:        durationFromSeconds(runtimeConfig.ShutdownGraceSeconds, 5),
		EffectiveConcurrency: effectivePluginConcurrency(snapshot.Concurrency, runtimeConfig.MaxConcurrentTasksPerPlugin),
	}, nil
}

func effectivePluginConcurrency(manifestConcurrency int, maxPerPlugin int) int {
	if manifestConcurrency < 1 {
		manifestConcurrency = 1
	}
	if maxPerPlugin < 1 {
		maxPerPlugin = 1
	}
	if manifestConcurrency > maxPerPlugin {
		return maxPerPlugin
	}
	return manifestConcurrency
}
