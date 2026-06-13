package plugins

const (
	ManifestValidationMaxSummary = 256

	validationMaxSummary = ManifestValidationMaxSummary
)

const (
	RegistrationStateInstalled = "installed"
	RegistrationStateRemoved   = "removed"

	DesiredStateEnabled  = "enabled"
	DesiredStateDisabled = "disabled"

	RuntimeStateStopped = "stopped"

	DisplayStateDiscovered      = "discovered"
	DisplayStateInvalidManifest = "invalid_manifest"
	DisplayStateConflict        = "conflict"

	CommandSourceManifest = "manifest"
	CommandSourceDynamic  = "dynamic"
)

const (
	stateInstalled    = RegistrationStateInstalled
	stateRemoved      = RegistrationStateRemoved
	stateDisabled     = DesiredStateDisabled
	stateStopped      = RuntimeStateStopped
	displayDiscovered = DisplayStateDiscovered
	displayInvalid    = DisplayStateInvalidManifest
	displayConflict   = DisplayStateConflict
)
