package protocolapi

import (
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
)

func (s *ProtocolService) ApplyConfigReload(cfg config.Config) error {
	if s == nil || s.adapter == nil {
		return nil
	}
	if s.adapter.Snapshot().State == adaptershell.StateStopped {
		return configruntime.ErrProtocolStopped
	}
	return s.adapter.Reload(cfg.OneBot, cfg.Adapter)
}
