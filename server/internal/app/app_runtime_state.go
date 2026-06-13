package app

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
