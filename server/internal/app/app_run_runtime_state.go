package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
)

func newAppRuntimeState(buildState appBuildState) *appRuntimeState {
	return &appRuntimeState{
		Config:               buildState.core.Config,
		Summary:              buildState.core.Summary,
		Logger:               buildState.core.Logger,
		LogLevel:             buildState.core.LogLevel,
		repoRoot:             buildState.core.repoRoot,
		redactText:           buildState.core.redactText,
		startedAt:            buildState.core.startedAt,
		startupRuntimeStates: newStartupRuntimeStates(nil),
	}
}

func (s *appRuntimeState) AuthConfig() managementhttp.AuthConfig {
	if s == nil {
		return managementhttp.AuthConfig{}
	}
	return managementhttp.AuthConfig{
		SetupLocalOnly:     s.Config.Web.SetupLocalOnly,
		LoginFailureLimit:  managementhttp.LoginFailureLimit(s.Config),
		LoginFailureWindow: managementhttp.LoginFailureWindow(s.Config),
	}
}

func (s *appRuntimeState) CurrentConfig() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.Config
}
