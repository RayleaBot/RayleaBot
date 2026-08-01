package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

type BotInfo struct {
	ID       string
	Nickname string
}

type InitPayload struct {
	Bot             BotInfo
	Capabilities    []string
	SuperAdmins     []string
	CommandPrefixes []string
}

type Spec struct {
	PluginID             string
	PluginName           string
	RepoRoot             string
	Runtime              string
	Command              string
	Args                 []string
	Env                  []string
	WorkDir              string
	EntryPath            string
	InitTimeout          time.Duration
	InitMaxTotal         time.Duration
	EventTimeout         time.Duration
	ShutdownGrace        time.Duration
	EffectiveConcurrency int
}

func BuildSpec(snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	return BuildSpecWithContext(context.Background(), snapshot, repoRoot, runtimeConfig)
}

func BuildSpecWithContext(ctx context.Context, snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	_ = ctx
	if snapshot.PluginID == "" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin_id is required for runtime startup", nil)
	}
	if !snapshot.Valid {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is not eligible for runtime startup", nil)
	}
	if snapshot.DisplayState == "conflict" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is conflicted and cannot be started", nil)
	}
	if snapshot.Runtime != "go" || snapshot.Entry == "" || snapshot.ManifestPath == "" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is missing runtime startup fields", nil)
	}
	packageRoot := snapshot.PackageRootPath
	if packageRoot == "" {
		packageRoot = filepath.Dir(resolveManifestPath(repoRoot, snapshot.ManifestPath))
	} else if !filepath.IsAbs(packageRoot) && repoRoot != "" {
		packageRoot = filepath.Join(repoRoot, filepath.FromSlash(packageRoot))
	}
	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		return Spec{}, errorf(codePluginPlatformMismatch, "plugin artifact does not support this host", err)
	}
	verified, err := artifact.Verify(packageRoot, artifact.Options{ExpectedPlatform: targetPlatform})
	if err != nil {
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return Spec{}, errorf(codePluginPlatformMismatch, "plugin artifact targets a different platform", err)
		}
		return Spec{}, errorf(codePluginArtifactInvalid, "plugin artifact validation failed", err)
	}
	if verified.Manifest.ID != snapshot.PluginID || verified.Manifest.Entry != snapshot.Entry {
		return Spec{}, errorf(codePluginArtifactInvalid, "plugin artifact does not match the catalog snapshot", nil)
	}

	initTimeout := durationFromSeconds(runtimeConfig.PluginInitTimeoutSeconds, 10)
	initMaxTotal := durationFromSeconds(runtimeConfig.PluginInitMaxTotalSeconds, 300)

	return Spec{
		PluginID:             snapshot.PluginID,
		PluginName:           snapshot.Name,
		RepoRoot:             repoRoot,
		Runtime:              snapshot.Runtime,
		Command:              verified.BackendPath,
		Args:                 nil,
		Env:                  nil,
		WorkDir:              verified.Root,
		EntryPath:            verified.BackendPath,
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

func resolveManifestPath(repoRoot, manifestPath string) string {
	if filepath.IsAbs(manifestPath) {
		return manifestPath
	}
	if repoRoot == "" {
		return filepath.Clean(filepath.FromSlash(manifestPath))
	}
	return filepath.Join(repoRoot, filepath.FromSlash(manifestPath))
}

func durationFromSeconds(seconds int, fallback int) time.Duration {
	if seconds <= 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}
