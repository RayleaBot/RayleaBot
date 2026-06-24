package httpwire

import (
	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire/configmodule"
	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire/routemodule"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
)

type RuntimeState = configmodule.RuntimeState
type ConfigDeps = configmodule.Deps
type ConfigService = configruntime.Service

type BuildDeps = routemodule.Deps
type State = routemodule.State
type Handlers = routemodule.Handlers

func Build(deps BuildDeps) State {
	return routemodule.Build(deps)
}

func NewConfigService(deps ConfigDeps) *ConfigService {
	return configmodule.NewService(deps)
}

func ClassifyConfigApplyEffects(oldCfg config.Config, newCfg config.Config) configapi.ApplyEffects {
	return configmodule.ClassifyApplyEffects(oldCfg, newCfg)
}

func ConfigDocumentFromTyped(cfg config.Config) map[string]any {
	return configmodule.ConfigDocumentFromTyped(cfg)
}
