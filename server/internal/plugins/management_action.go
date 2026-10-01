package plugins

import "errors"

// ErrRuntimeNotRunning marks host admission rejection before a frame is written.
// A plugin-provided error code cannot manufacture this identity.
var ErrRuntimeNotRunning = errors.New("plugin runtime did not admit the action")

type NotRunningError struct{ PluginID, State string }

func (e *NotRunningError) Error() string { return "plugin is not running: " + e.PluginID }
func (e *NotRunningError) Details() map[string]any {
	return map[string]any{"plugin_id": e.PluginID, "state": e.State}
}
