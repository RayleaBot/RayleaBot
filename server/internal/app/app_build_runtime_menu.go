package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func buildRuntimeRegistryForApp(buildState appBuildState, runtimeState *appRuntimeState, platform appPlatform, localActions *localaction.Service) *runtimeRegistry {
	return newRuntimeRegistry(runtimeState.Logger, runtime.Options{
		Console:                    platform.Console,
		RedactText:                 buildState.managementRedact,
		StderrRateLimitBytesPerSec: buildState.core.Config.Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         localActions.Execute,
	})
}

func buildBuiltinMenuService(runtimeState *appRuntimeState, pluginStack appPlugins) *menuext.Service {
	return menuext.New(menuext.Deps{
		CurrentConfig: func() config.Config { return runtimeState.Config },
		Plugins:       pluginStack.Plugins,
		Renderer:      pluginStack.renderer,
		Sender:        pluginStack.outboundSender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if pluginStack.outboundLimiter == nil {
				return nil
			}
			return pluginStack.outboundLimiter.Wait(ctx, request)
		},
		Logger: runtimeState.Logger,
	})
}
