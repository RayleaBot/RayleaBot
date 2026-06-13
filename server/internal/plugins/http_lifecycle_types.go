package plugins

import "context"

type DesiredStateController interface {
	Enable(context.Context, string) (Snapshot, error)
	Disable(context.Context, string) (Snapshot, error)
	Reload(context.Context, string) (Snapshot, error)
	RecoverFromDeadLetter(context.Context, string) (Snapshot, error)
}
