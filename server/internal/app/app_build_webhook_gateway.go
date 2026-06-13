package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
)

func buildPluginWebhookGateway(
	runtimeState *appRuntimeState,
	platform appPlatform,
	pluginStack appPlugins,
	lifecycle *pluginLifecycleController,
	grantView *pluginGrantView,
) *pluginwebhook.Service {
	return pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: func() config.Config { return runtimeState.Config },
		Logger:        runtimeState.Logger,
		Registry:      pluginStack.webhooks,
		Secrets:       platform.Secrets,
		Plugins:       pluginStack.Plugins,
		Dispatcher:    pluginStack.Dispatcher,
		Runtime:       lifecycle,
		Grants:        grantView,
	})
}
