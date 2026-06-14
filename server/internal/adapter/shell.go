package adapter

import (
	"log/slog"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type Shell = adaptershell.Shell
type MetricsObserver = adaptershell.MetricsObserver

func New(cfg config.OneBotConfig, adapterCfg config.AdapterConfig, logger *slog.Logger) *Shell {
	return adaptershell.New(cfg, adapterCfg, logger)
}

func NewForTest(cfg config.OneBotConfig, adapterCfg config.AdapterConfig, logger *slog.Logger, skipRuntimeInfo bool) *Shell {
	return adaptershell.NewForTest(cfg, adapterCfg, logger, skipRuntimeInfo)
}
