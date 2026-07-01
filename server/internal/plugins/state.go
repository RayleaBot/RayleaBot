package plugins

import "time"

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

	PluginStateDisabled = "disabled"
	PluginStateEnabled  = "enabled"
	PluginStateStarting = "starting"
	PluginStateRunning  = "running"
	PluginStateStopping = "stopping"
	PluginStateFailed   = "failed"
	PluginStateInvalid  = "invalid"

	StateDiagnosisInvalidManifest  = "invalid_manifest"
	StateDiagnosisPluginIDConflict = "plugin_id_conflict"
	StateDiagnosisCrashed          = "crashed"
	StateDiagnosisRetrying         = "retrying"
	StateDiagnosisRecoveryRequired = "recovery_required"

	DisplayStateDiscovered      = "discovered"
	DisplayStateInvalidManifest = "invalid_manifest"
	DisplayStateConflict        = "conflict"

	CommandSourceManifest = "manifest"
	CommandSourceDynamic  = "dynamic"
	CommandSourcePattern  = "pattern"
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

const (
	displayRemoved    = "removed"
	displayEnabled    = "enabled"
	displayEnabling   = "enabling"
	displayRunning    = "running"
	displayDisabling  = "disabling"
	displayStopping   = "stopping"
	displayCrashed    = "crashed"
	displayBackoff    = "backoff"
	displayDeadLetter = "dead_letter"
	displayDisabled   = "disabled"
)

type StateDiagnosis struct {
	Kind             string     `json:"kind"`
	Summary          string     `json:"summary,omitempty"`
	ManifestPath     string     `json:"manifest_path,omitempty"`
	ManifestPaths    []string   `json:"manifest_paths,omitempty"`
	SourceRoots      []string   `json:"source_roots,omitempty"`
	LastErrorCode    string     `json:"last_error_code,omitempty"`
	LastErrorMessage string     `json:"last_error_message,omitempty"`
	CrashCount       int        `json:"crash_count,omitempty"`
	EnteredAt        *time.Time `json:"entered_at,omitempty"`
	RetryAt          *time.Time `json:"retry_at,omitempty"`
	Recoverable      bool       `json:"recoverable,omitempty"`
}

func ProjectState(snapshot Snapshot) (string, *StateDiagnosis) {
	if snapshot.RegistrationState == stateRemoved {
		return PluginStateDisabled, nil
	}

	if !snapshot.Valid || snapshot.RegistrationState != stateInstalled {
		diagnosis := &StateDiagnosis{
			Kind:         StateDiagnosisInvalidManifest,
			Summary:      snapshot.ValidationSummary,
			ManifestPath: snapshot.ManifestPath,
			Recoverable:  false,
		}
		if snapshot.DisplayState == displayConflict || len(snapshot.ConflictPaths) > 0 {
			diagnosis.Kind = StateDiagnosisPluginIDConflict
			diagnosis.ManifestPaths = append([]string(nil), snapshot.ConflictPaths...)
			diagnosis.SourceRoots = append([]string(nil), snapshot.SourceRoots...)
			diagnosis.ManifestPath = ""
		}
		return PluginStateInvalid, diagnosis
	}

	switch snapshot.RuntimeState {
	case "starting":
		return PluginStateStarting, nil
	case "running":
		return PluginStateRunning, nil
	case "stopping":
		return PluginStateStopping, nil
	case "crashed":
		return PluginStateFailed, &StateDiagnosis{
			Kind:        StateDiagnosisCrashed,
			Summary:     "插件运行时异常退出",
			Recoverable: false,
		}
	case "backoff":
		return PluginStateFailed, &StateDiagnosis{
			Kind:        StateDiagnosisRetrying,
			Summary:     "插件运行时正在等待重试",
			Recoverable: false,
		}
	case "dead_letter":
		diagnosis := &StateDiagnosis{
			Kind:        StateDiagnosisRecoveryRequired,
			Summary:     "插件运行时需要人工恢复",
			Recoverable: true,
		}
		if snapshot.DeadLetter != nil {
			enteredAt := snapshot.DeadLetter.EnteredAt
			diagnosis.EnteredAt = &enteredAt
			diagnosis.CrashCount = snapshot.DeadLetter.CrashCount
			diagnosis.LastErrorCode = snapshot.DeadLetter.LastErrorCode
			diagnosis.LastErrorMessage = snapshot.DeadLetter.LastErrorMessage
		}
		return PluginStateFailed, diagnosis
	}

	if snapshot.DesiredState == DesiredStateEnabled {
		return PluginStateEnabled, nil
	}
	return PluginStateDisabled, nil
}

func projectDisplayState(snapshot Snapshot) string {
	if snapshot.RegistrationState == stateRemoved {
		return displayRemoved
	}

	switch snapshot.DisplayState {
	case displayDiscovered, displayInvalid, displayConflict,
		displayEnabled, displayEnabling, displayRunning,
		displayDisabling, displayStopping, displayCrashed,
		displayBackoff, displayDeadLetter, displayDisabled:
		return snapshot.DisplayState
	}

	if snapshot.DisplayState != "" {
		return displayInvalid
	}

	if !snapshot.Valid || snapshot.RegistrationState != stateInstalled {
		return displayInvalid
	}

	return defaultDisplayState(snapshot)
}

func defaultDisplayState(snapshot Snapshot) string {
	if snapshot.RegistrationState == stateRemoved {
		return displayRemoved
	}
	if !snapshot.Valid || snapshot.RegistrationState != stateInstalled {
		return displayInvalid
	}

	switch snapshot.RuntimeState {
	case "starting":
		return displayEnabling
	case "running":
		return displayRunning
	case "stopping":
		if snapshot.DesiredState == "disabled" {
			return displayDisabling
		}
		return displayStopping
	case "crashed":
		return displayCrashed
	case "backoff":
		return displayBackoff
	case "dead_letter":
		return displayDeadLetter
	}

	if snapshot.DesiredState == "enabled" {
		return displayEnabled
	}
	return displayDisabled
}

func DefaultDisplayState(snapshot Snapshot) string {
	return defaultDisplayState(snapshot)
}

func ApplyDesiredStates(snapshots []Snapshot, states map[string]string) []Snapshot {
	if len(snapshots) == 0 {
		return nil
	}
	result := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		cloned := CloneSnapshot(snapshot)
		if desired, ok := states[cloned.PluginID]; ok &&
			cloned.RegistrationState == stateInstalled &&
			(desired == "enabled" || desired == "disabled") {
			cloned.DesiredState = desired
			cloned.DisplayState = defaultDisplayState(cloned)
		}
		result = append(result, cloned)
	}
	return result
}
