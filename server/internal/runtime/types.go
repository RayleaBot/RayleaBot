package runtime

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
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
	Subscriptions       []string
}

// CrashCallback is invoked by the runtime manager when a running plugin
// process exits unexpectedly. The lifecycle controller uses this to drive
// the backoff/restart cycle.
type CrashCallback func(pluginID string, crashCount int, lastErrorCode string)

type managerDeps struct {
	now       func() time.Time
	requestID func() string
}

type LocalActionExecutor func(context.Context, string, string, Action, Event) (map[string]any, error)

type Options struct {
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	OnCrash                    CrashCallback
	ExecuteLocalAction         LocalActionExecutor
}
