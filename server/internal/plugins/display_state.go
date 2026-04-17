package plugins

import "strings"

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

	switch strings.TrimSpace(snapshot.DisplayState) {
	case displayDiscovered, displayInvalid, displayConflict:
		return snapshot.DisplayState
	}

	if !snapshot.Valid {
		return displayInvalid
	}

	return defaultDisplayState(snapshot)
}
