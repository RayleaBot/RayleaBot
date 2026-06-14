package pluginapi

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type DesiredStateController interface {
	Enable(context.Context, string) (plugins.Snapshot, error)
	Disable(context.Context, string) (plugins.Snapshot, error)
	Reload(context.Context, string) (plugins.Snapshot, error)
	RecoverFromDeadLetter(context.Context, string) (plugins.Snapshot, error)
}
