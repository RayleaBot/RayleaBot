package plugins

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
