package configmodule

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type RuntimeState interface {
	CurrentConfig() config.Config
	CurrentSummary() config.Summary
	SetConfig(config.Config)
	SetSummary(config.Summary)
	RuntimeLogger() *slog.Logger
	RuntimeLogLevel() *logging.LevelController
	RepoRoot() string
	AddRedactionValues(...string)
}

type Deps struct {
	Runtime          RuntimeState
	Logs             *logging.Stream
	LogRepository    logging.Repository
	Renderer         *renderservice.Service
	PluginLogLimiter *localaction.PluginLogLimiter
	OutboundLimiter  interface{ ApplyConfig(config.Config) }
	Protocol         *protocolapi.Service
	EventIngress     *eventingress.Service
	Secrets          secrets.Store
}

func NewService(deps Deps) *configruntime.Service {
	runtimeDeps := configruntime.Deps{
		CurrentConfig: func() config.Config {
			if deps.Runtime == nil {
				return config.Config{}
			}
			return deps.Runtime.CurrentConfig()
		},
		CurrentSummary: func() config.Summary {
			if deps.Runtime == nil {
				return config.Summary{}
			}
			return deps.Runtime.CurrentSummary()
		},
		SetConfig: func(cfg config.Config) {
			if deps.Runtime != nil {
				deps.Runtime.SetConfig(cfg)
			}
		},
		SetSummary: func(summary config.Summary) {
			if deps.Runtime != nil {
				deps.Runtime.SetSummary(summary)
			}
		},
		Logger:        runtimeStateLogger(deps.Runtime),
		LogLevel:      runtimeStateLogLevel(deps.Runtime),
		Logs:          deps.Logs,
		LogRepository: deps.LogRepository,
		AddRedactionValues: func(values ...string) {
			if deps.Runtime != nil {
				deps.Runtime.AddRedactionValues(values...)
			}
		},
		Renderer:         deps.Renderer,
		PluginLogLimiter: deps.PluginLogLimiter,
		OutboundLimiter:  deps.OutboundLimiter,
		EventIngress:     deps.EventIngress,
		Secrets:          deps.Secrets,
	}
	if deps.Protocol != nil {
		runtimeDeps.Protocol = deps.Protocol
	}
	return configruntime.NewService(runtimeDeps)
}

func runtimeStateLogger(state RuntimeState) *slog.Logger {
	if state == nil {
		return nil
	}
	return state.RuntimeLogger()
}

func runtimeStateLogLevel(state RuntimeState) *logging.LevelController {
	if state == nil {
		return nil
	}
	return state.RuntimeLogLevel()
}

func ClassifyApplyEffects(oldCfg config.Config, newCfg config.Config) configapi.ApplyEffects {
	return configruntime.ClassifyApplyEffects(oldCfg, newCfg)
}

func ConfigDocumentFromTyped(cfg config.Config) map[string]any {
	return configruntime.ConfigDocumentFromTyped(cfg)
}
