package plugins

import "errors"

var (
	ErrPluginNotFound        = errors.New("plugin not found")
	ErrStateConflict         = errors.New("state conflict")
	ErrPluginNotInDeadLetter = errors.New("plugin is not in dead_letter")
)

type PermissionPendingError struct {
	PluginID            string
	MissingCapabilities []string
	ScopeChanged        bool
}

func (e *PermissionPendingError) Error() string {
	return "plugin permission pending"
}
