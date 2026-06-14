package managementhttp

import pluginscore "github.com/RayleaBot/RayleaBot/server/internal/plugins"

type CatalogView = pluginscore.CatalogView
type Snapshot = pluginscore.Snapshot
type DesiredStateRepository = pluginscore.DesiredStateRepository
type InstallRequest = pluginscore.InstallRequest
type InstallCoordinator = pluginscore.InstallCoordinator
type PluginGrant = pluginscore.PluginGrant
type GrantRepository = pluginscore.GrantRepository
type EffectiveGrant = pluginscore.EffectiveGrant
type PermissionSummary = pluginscore.PermissionSummary
type GrantSource = pluginscore.GrantSource
type PermissionRequirement = pluginscore.PermissionRequirement
type PermissionStatus = pluginscore.PermissionStatus
type PermissionSource = pluginscore.PermissionSource
type PermissionPendingError = pluginscore.PermissionPendingError
type Help = pluginscore.Help
type HelpGroup = pluginscore.HelpGroup
type HelpItem = pluginscore.HelpItem
type HelpView = pluginscore.HelpView
type HelpGroupView = pluginscore.HelpGroupView
type HelpItemView = pluginscore.HelpItemView
type WebhookScope = pluginscore.WebhookScope
type Screenshot = pluginscore.Screenshot
type ManagementUI = pluginscore.ManagementUI
type ManagementUIPage = pluginscore.ManagementUIPage
type RenderTemplate = pluginscore.RenderTemplate

const (
	CommandSourceManifest = pluginscore.CommandSourceManifest
	CommandSourceDynamic  = pluginscore.CommandSourceDynamic
	displayConflict       = pluginscore.DisplayStateConflict

	GrantSourceBuiltinAuto = pluginscore.GrantSourceBuiltinAuto
	GrantSourceConfigAuto  = pluginscore.GrantSourceConfigAuto
	GrantSourcePersisted   = pluginscore.GrantSourcePersisted

	PermissionRequirementRequired = pluginscore.PermissionRequirementRequired
	PermissionRequirementOptional = pluginscore.PermissionRequirementOptional

	PermissionStatusGranted    = pluginscore.PermissionStatusGranted
	PermissionStatusNotGranted = pluginscore.PermissionStatusNotGranted

	PermissionSourceBuiltinAuto = pluginscore.PermissionSourceBuiltinAuto
	PermissionSourceConfigAuto  = pluginscore.PermissionSourceConfigAuto
	PermissionSourcePersisted   = pluginscore.PermissionSourcePersisted
	PermissionSourceNone        = pluginscore.PermissionSourceNone
)

var (
	ErrPluginNotFound        = pluginscore.ErrPluginNotFound
	ErrStateConflict         = pluginscore.ErrStateConflict
	ErrPluginNotInDeadLetter = pluginscore.ErrPluginNotInDeadLetter
)

func DetectCommandConflicts(snapshots []Snapshot) map[string][]string {
	return pluginscore.DetectCommandConflicts(snapshots)
}

func ComputeEffectiveGrants(snapshot Snapshot, configAutoCapabilities []string, persisted []PluginGrant) []EffectiveGrant {
	return pluginscore.ComputeEffectiveGrants(snapshot, configAutoCapabilities, persisted)
}

func BuildPermissionSummaries(snapshot Snapshot, effectiveGrants []EffectiveGrant) []PermissionSummary {
	return pluginscore.BuildPermissionSummaries(snapshot, effectiveGrants)
}

func CloneSnapshot(snapshot Snapshot) Snapshot {
	return pluginscore.CloneSnapshot(snapshot)
}

func DefaultDisplayState(snapshot Snapshot) string {
	return pluginscore.DefaultDisplayState(snapshot)
}

func cloneMap(values map[string]any) map[string]any {
	return pluginscore.CloneMap(values)
}

func dedupeCapabilities(values []string) []string {
	return pluginscore.DedupeCapabilities(values)
}

func buildHelpView(snapshot Snapshot) *pluginscore.HelpView {
	return pluginscore.BuildHelpView(snapshot)
}
