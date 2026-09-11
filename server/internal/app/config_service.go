package app

import (
	"log/slog"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	configruntime "github.com/RayleaBot/RayleaBot/server/internal/config/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

type configRuntimeState interface {
	CurrentConfig() config.Config
	CurrentSummary() config.Summary
	SetConfig(config.Config)
	SetSummary(config.Summary)
	RuntimeLogger() *slog.Logger
	RuntimeLogLevel() *logging.LevelController
	RepoRoot() string
	AddRedactionValues(...string)
}

type configServiceDeps struct {
	EffectiveTimezone func() string
	Runtime           configRuntimeState
	Logs              *logging.Stream
	LogRepository     logging.Repository
	Renderer          *render.Service
	PluginLogLimiter  *localaction.PluginLogLimiter
	OutboundLimiter   interface{ ApplyConfig(config.Config) }
	AccountValidation interface{ ApplyConfig(config.Config) }
	Protocol          *adapterservice.Service
	EventIngress      *chatpolicy.Ingress
	Secrets           secrets.Store
}

func newConfigService(deps configServiceDeps) *configruntime.Service {
	runtimeDeps := configruntime.Deps{
		EffectiveTimezone: deps.EffectiveTimezone,
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
		PluginLogLimiter:  deps.PluginLogLimiter,
		OutboundLimiter:   deps.OutboundLimiter,
		AccountValidation: deps.AccountValidation,
		Secrets:           deps.Secrets,
	}
	// Assign concrete pointers only when non-nil so interface deps stay nil
	// instead of holding typed nils.
	if deps.Renderer != nil {
		runtimeDeps.Renderer = deps.Renderer
	}
	if deps.EventIngress != nil {
		runtimeDeps.EventIngress = deps.EventIngress
	}
	if deps.Protocol != nil {
		runtimeDeps.Protocol = deps.Protocol
	}
	return configruntime.NewService(runtimeDeps)
}

func runtimeStateLogger(state configRuntimeState) *slog.Logger {
	if state == nil {
		return nil
	}
	return state.RuntimeLogger()
}

func runtimeStateLogLevel(state configRuntimeState) *logging.LevelController {
	if state == nil {
		return nil
	}
	return state.RuntimeLogLevel()
}
