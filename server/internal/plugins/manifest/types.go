package pluginmanifest

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

const (
	ManifestValidationMaxSummary = plugins.ManifestValidationMaxSummary

	RegistrationStateInstalled  = plugins.RegistrationStateInstalled
	DesiredStateEnabled         = plugins.DesiredStateEnabled
	DesiredStateDisabled        = plugins.DesiredStateDisabled
	RuntimeStateStopped         = plugins.RuntimeStateStopped
	DisplayStateDiscovered      = plugins.DisplayStateDiscovered
	DisplayStateInvalid         = plugins.DisplayStateInvalidManifest
	DisplayStateInvalidManifest = plugins.DisplayStateInvalidManifest
	DisplayStateConflict        = plugins.DisplayStateConflict

	CommandSourceManifest = plugins.CommandSourceManifest
	CommandSourceDynamic  = plugins.CommandSourceDynamic
)

func cloneMap(values map[string]any) map[string]any {
	return plugins.CloneSettings(values)
}

func cloneValue(value any) any {
	return plugins.CloneSettingValue(value)
}
