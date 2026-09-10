package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

type InitPayload struct {
	Timezone        string
	Bots            []chatevent.BotIdentity
	Config          map[string]any
	Permissions     []string
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
	Events               []string
	InitTimeout          time.Duration
	EventTimeout         time.Duration
	ShutdownGrace        time.Duration
	EffectiveConcurrency int
	IPCPendingActionsMax int
	IPCActionBurstCount  int
	IPCActionBurstWindow time.Duration
	IPCMessageMaxBytes   int
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
	if snapshot.ManifestPath == "" {
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
		if errors.Is(err, artifact.ErrContractUnsupported) {
			return Spec{}, errorf(errorcodes.PluginContractUnsupported, "plugin contract version is unsupported", err)
		}
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return Spec{}, errorf(codePluginPlatformMismatch, "plugin artifact targets a different platform", err)
		}
		return Spec{}, errorf(codePluginArtifactInvalid, "plugin artifact validation failed", err)
	}
	if verified.Manifest.ID != snapshot.PluginID {
		return Spec{}, errorf(codePluginArtifactInvalid, "plugin artifact does not match the catalog snapshot", nil)
	}

	initTimeout := durationFromSeconds(runtimeConfig.PluginInitTimeoutSeconds, 10)
	burstLimit, err := config.ParseRateLimit(runtimeConfig.IPCActionBurstLimit)
	if err != nil {
		burstLimit, _ = config.ParseRateLimit("100/1s")
	}

	return Spec{
		PluginID:             snapshot.PluginID,
		PluginName:           snapshot.Name,
		RepoRoot:             repoRoot,
		Runtime:              "native",
		Command:              verified.BackendPath,
		Args:                 nil,
		Env:                  managedRuntimeEnvironment(repoRoot),
		WorkDir:              verified.Root,
		EntryPath:            verified.BackendPath,
		Events:               append([]string(nil), snapshot.Events...),
		InitTimeout:          initTimeout,
		EventTimeout:         durationFromSeconds(runtimeConfig.PluginEventTimeoutSeconds, 5),
		ShutdownGrace:        durationFromSeconds(runtimeConfig.ShutdownGraceSeconds, 5),
		EffectiveConcurrency: effectivePluginConcurrency(snapshot.Concurrency, runtimeConfig.MaxConcurrentTasksPerPlugin),
		IPCPendingActionsMax: positiveInt(runtimeConfig.IPCPendingActionsMax, 256),
		IPCActionBurstCount:  burstLimit.Count,
		IPCActionBurstWindow: burstLimit.Window,
		IPCMessageMaxBytes:   positiveInt(runtimeConfig.IPCMessageMaxBytes, 8*1024*1024),
	}, nil
}

func managedRuntimeEnvironment(repoRoot string) []string {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	runtimeDeps := deps.NewRuntime(repoRoot)
	bindings := []struct {
		name       string
		entrypoint string
	}{
		{name: "RAYLEABOT_FFMPEG_PATH", entrypoint: "ffmpeg"},
		{name: "RAYLEABOT_FFPROBE_PATH", entrypoint: "ffprobe"},
	}
	environment := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		path, err := runtimeDeps.ResolvePreparedEntrypoint("ffmpeg", binding.entrypoint)
		if err != nil || strings.TrimSpace(path) == "" {
			continue
		}
		environment = append(environment, binding.name+"="+path)
	}
	return environment
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

func positiveInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
