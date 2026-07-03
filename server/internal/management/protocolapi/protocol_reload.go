package protocolapi

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

func (s *ProtocolService) ApplyConfigReload(cfg config.Config) error {
	if s == nil || s.adapter == nil {
		return nil
	}
	if s.adapter.Snapshot().State == onebot11.StateStopped {
		return configruntime.ErrProtocolStopped
	}
	return s.adapter.Reload(cfg.OneBot, cfg.Adapter)
}
