package managementhttp

import (
	"errors"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

var ErrProtocolStopped = errors.New("protocol adapter stopped")

func (s *ProtocolService) ApplyConfigReload(cfg config.Config) error {
	if s == nil || s.adapter == nil {
		return nil
	}
	if s.adapter.Snapshot().State == adaptershell.StateStopped {
		return ErrProtocolStopped
	}
	return s.adapter.Reload(cfg.OneBot, cfg.Adapter)
}
