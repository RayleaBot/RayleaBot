package servicegraph

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
)

type runtimeRegistryDeps struct {
	Logger                     *slog.Logger
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	ExecuteLocalAction         runtimemanager.LocalActionExecutor
}

func buildRuntimeRegistry(deps runtimeRegistryDeps) *runtimeregistry.Registry {
	return runtimeregistry.New(deps.Logger, runtimemanager.Options{
		Console:                    deps.Console,
		RedactText:                 deps.RedactText,
		StderrRateLimitBytesPerSec: deps.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         deps.ExecuteLocalAction,
	})
}
