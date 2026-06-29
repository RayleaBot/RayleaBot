package pluginmodule

import (
	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/actionwiring"
	plugincapabilityview "github.com/RayleaBot/RayleaBot/server/internal/plugins/capabilityview"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func buildPluginCapabilityView(pluginStack pluginstack.State, eventStack eventstack.State) *plugincapabilityview.View {
	capabilityView := plugincapabilityview.New(plugincapabilityview.Deps{
		Plugins: pluginStack.Plugins,
	})
	if eventStack.Dispatcher != nil {
		eventStack.Dispatcher.SetCapabilityChecker(capabilityView.CapabilityDeclared)
	}
	return capabilityView
}

func buildLocalActionService(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	renderer *renderservice.Service,
	capabilityView *plugincapabilityview.View,
	governanceService *governance.Service,
	thirdParty localaction.ThirdPartyAccountReader,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		RedactText:       runtimeState.RedactString,
		Capabilities:     capabilityView,
		PluginConfig:     pluginStack.PluginConfig,
		PluginFiles:      pluginStack.PluginFiles,
		PluginKV:         pluginStack.PluginKV,
		Secrets:          actionwiring.SecretReaderFromStore(platform.Secrets),
		ThirdParty:       thirdParty,
		Scheduler:        actionwiring.Scheduler(platform.Scheduler),
		Dispatcher:       actionwiring.ConfigChangedDispatcher(eventStack.Dispatcher),
		Renderer:         actionwiring.RendererFromService(renderer),
		Adapter:          eventStack.Adapter,
		PluginLogLimiter: pluginStack.PluginLogLimiter,
		Governance:       governanceService,
		RefreshCommands:  actionwiring.RefreshCommands(pluginStack.Plugins, eventStack.Dispatcher),
		Registrars:       actionwiring.DefaultRegistrars(),
	})
}
