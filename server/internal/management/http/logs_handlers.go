package managementhttp

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

type LogService interface {
	CurrentBootID() string
	ListLogPage(context.Context, logging.PageQuery) (logging.PageResult, error)
	GetLogSummary(context.Context, string) (logging.Summary, error)
}

type LogHandlers struct {
	logs LogService
}

func NewLogHandlers(logs LogService) *LogHandlers {
	return &LogHandlers{logs: logs}
}
