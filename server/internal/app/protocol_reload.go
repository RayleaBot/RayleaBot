package app

import (
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

var errProtocolStopped = errors.New("protocol adapter stopped")

func (s *protocolService) ApplyConfigReload(cfg config.Config) error {
	if s == nil || s.adapter == nil {
		return nil
	}
	if s.adapter.Snapshot().State == adapter.StateStopped {
		return errProtocolStopped
	}
	return s.adapter.Reload(cfg.OneBot, cfg.Adapter)
}
