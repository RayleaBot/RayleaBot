package runtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
)

const DefaultMaxCrashRetries = runtimemanager.DefaultMaxCrashRetries

type Manager = runtimemanager.Manager

func New(logger *slog.Logger, options Options) *Manager {
	return runtimemanager.New(logger, options)
}

func BuildSpec(snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	return runtimemanager.BuildSpec(snapshot, repoRoot, runtimeConfig)
}

func BuildSpecWithContext(ctx context.Context, snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	return runtimemanager.BuildSpecWithContext(ctx, snapshot, repoRoot, runtimeConfig)
}

func CrashBackoff(crashCount, initialSeconds, maxSeconds int) time.Duration {
	return runtimemanager.CrashBackoff(crashCount, initialSeconds, maxSeconds)
}
