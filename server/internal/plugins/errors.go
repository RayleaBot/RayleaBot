package plugins

import "errors"

var (
	ErrPluginNotFound        = errors.New("plugin not found")
	ErrStateConflict         = errors.New("state conflict")
	ErrPluginNotInDeadLetter = errors.New("plugin is not in dead_letter")
)
