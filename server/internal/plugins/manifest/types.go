package pluginmanifest

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

const (
	ManifestValidationMaxSummary = plugins.ManifestValidationMaxSummary

	RegistrationStateInstalled = plugins.RegistrationStateInstalled
	DesiredStateEnabled        = plugins.DesiredStateEnabled
	DesiredStateDisabled       = plugins.DesiredStateDisabled
	RuntimeStateStopped        = plugins.RuntimeStateStopped
	DisplayStateDiscovered     = plugins.DisplayStateDiscovered
	DisplayStateInvalid        = plugins.DisplayStateInvalidManifest
	DisplayStateInvalidManifest = plugins.DisplayStateInvalidManifest
	DisplayStateConflict       = plugins.DisplayStateConflict

	CommandSourceManifest = plugins.CommandSourceManifest
	CommandSourceDynamic  = plugins.CommandSourceDynamic
)

type Snapshot = plugins.Snapshot
type Command = plugins.Command
type DynamicCommandDecl = plugins.DynamicCommandDecl
type WebhookScope = plugins.WebhookScope
type Screenshot = plugins.Screenshot
type ManagementUI = plugins.ManagementUI
type ManagementUIPage = plugins.ManagementUIPage
type RenderTemplate = plugins.RenderTemplate
type Help = plugins.Help
type HelpGroup = plugins.HelpGroup
type HelpItem = plugins.HelpItem

func cloneMap(values map[string]any) map[string]any {
	return plugins.CloneSettings(values)
}

func cloneValue(value any) any {
	return plugins.CloneSettingValue(value)
}
