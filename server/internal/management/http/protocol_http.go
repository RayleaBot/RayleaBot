package managementhttp

import (
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
)

type ConfigSource interface {
	CurrentConfig() config.Config
}

type ProtocolService struct {
	config      ConfigSource
	adapter     *adapter.Shell
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan managementevents.Frame
}

func NewProtocolService(configSource ConfigSource, adapterShell *adapter.Shell) *ProtocolService {
	return &ProtocolService{
		config:      configSource,
		adapter:     adapterShell,
		subscribers: make(map[uint64]chan managementevents.Frame),
	}
}
