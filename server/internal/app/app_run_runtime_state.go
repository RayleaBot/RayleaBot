package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
)

func newAppRuntimeState(buildState appBuildState) *appRuntimeState {
	return &appRuntimeState{
		Config:     buildState.core.Config,
		Summary:    buildState.core.Summary,
		Logger:     buildState.core.Logger,
		LogLevel:   buildState.core.LogLevel,
		repoRoot:   buildState.core.repoRoot,
		redactText: buildState.core.redactText,
		startedAt:  buildState.core.startedAt,
	}
}

func (s *appRuntimeState) AuthConfig() authapi.Config {
	if s == nil {
		return authapi.Config{}
	}
	return authapi.Config{
		SetupLocalOnly:     s.Config.Web.SetupLocalOnly,
		LoginFailureLimit:  authapi.LoginFailureLimit(s.Config),
		LoginFailureWindow: authapi.LoginFailureWindow(s.Config),
	}
}

func (s *appRuntimeState) CurrentConfig() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.Config
}
