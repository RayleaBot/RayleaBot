package runtime

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/console"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type State string

const (
	StateStopped    State = "stopped"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateStopping   State = "stopping"
	StateCrashed    State = "crashed"
	StateBackoff    State = "backoff"
	StateDeadLetter State = "dead_letter"
)

const (
	codePlatformInvalidRequest  = errorcodes.PlatformInvalidRequest
	codePlatformRateLimited     = errorcodes.PlatformRateLimited
	codePlatformResourceMissing = errorcodes.PlatformResourceMissing
	codePluginInitTimeout       = errorcodes.PluginInitTimeout
	codePluginEventTimeout      = errorcodes.PluginEventTimeout
	codePluginEventCanceled     = errorcodes.PluginEventCanceled
	codePluginInternalError     = errorcodes.PluginInternalError
	codePluginNotHandled        = errorcodes.PluginNotHandled
	codePluginProtocolViolation = errorcodes.PluginProtocolViolation
	codePluginShutdownTimeout   = errorcodes.PluginShutdownTimeout
	codePluginStopping          = errorcodes.PluginStopping
	codePluginArtifactInvalid   = errorcodes.PluginArtifactInvalid
	codePluginPlatformMismatch  = errorcodes.PluginPlatformMismatch
)

func errorf(code, message string, err error) *plugins.Error {
	return &plugins.Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func errorWithDetails(code, message string, details map[string]any, err error) *plugins.Error {
	return &plugins.Error{
		Code:    code,
		Message: message,
		Details: cloneDetails(details),
		Err:     err,
	}
}

func cloneDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = value
	}
	return cloned
}

type Snapshot struct {
	PluginID            string
	State               State
	LastErrorCode       string
	LastErrorMessage    string
	InitRequestID       string
	PID                 int
	StartedAt           *time.Time
	StoppedAt           *time.Time
	CrashCount          int
	NextRetryAt         *time.Time
	EnteredDeadLetterAt *time.Time
}

type InitResponseStatus int

const (
	InitResponseWait InitResponseStatus = iota
	InitResponseReady
)

// CrashCallback is invoked by the runtime manager when a running plugin
// process exits unexpectedly. The lifecycle controller uses this to drive
// the backoff/restart cycle.
type CrashCallback func(pluginID string, crashCount int, lastErrorCode string)

type managerDeps struct {
	now       func() time.Time
	requestID func() string
}

type LocalActionExecutor func(context.Context, string, string, plugins.Action, chatevent.Event) (map[string]any, error)

type Options struct {
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	OnCrash                    CrashCallback
	ExecuteLocalAction         LocalActionExecutor
}
