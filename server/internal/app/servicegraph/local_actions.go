package servicegraph

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/app/actionwire"
	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	bilibilicredential "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/credential"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincapabilityview "github.com/RayleaBot/RayleaBot/server/internal/plugins/capabilityview"
	lifecyclecommands "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle/commands"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func buildLocalActionService(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	renderer *renderservice.Service,
	capabilityView *plugincapabilityview.View,
	governanceService *governance.Service,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		RedactText:       runtimeState.RedactString,
		Capabilities:     capabilityView,
		PluginConfig:     pluginStack.PluginConfig,
		PluginFiles:      pluginStack.PluginFiles,
		PluginKV:         pluginStack.PluginKV,
		Secrets:          actionwire.SecretReader(platform.Secrets),
		Scheduler:        actionwire.Scheduler(platform.Scheduler),
		Dispatcher:       actionwire.ConfigChangedDispatcher(eventStack.Dispatcher),
		Renderer:         actionwire.Renderer(renderer),
		Adapter:          eventStack.Adapter,
		PluginLogLimiter: pluginStack.PluginLogLimiter,
		Governance:       governanceService,
		HTTPCredentials:  bilibilicredential.NewInjector(thirdPartyService, bilibiliSession),
	})
}

func configureLocalActionService(localActions *localaction.Service, pluginStack pluginstack.State, eventStack eventstack.State) {
	localActions.SetRefreshPluginCommands(func(ctx context.Context, pluginID string, settings map[string]any) {
		lifecyclecommands.RefreshPluginCommands(pluginStack.Plugins, eventStack.Dispatcher, pluginID, settings)
	})
}
