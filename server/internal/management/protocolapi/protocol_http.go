package protocolapi

import (
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

type ConfigSource interface {
	CurrentConfig() config.Config
}

type ProtocolService struct {
	config                    ConfigSource
	adapter                   *onebot11.Shell
	oneBot11TargetReadTimeout time.Duration
	mu                        sync.RWMutex
	nextSubID                 uint64
	subscribers               map[uint64]chan managementevents.Frame
}

func NewProtocolService(configSource ConfigSource, adapterShell *onebot11.Shell) *ProtocolService {
	return &ProtocolService{
		config:                    configSource,
		adapter:                   adapterShell,
		oneBot11TargetReadTimeout: 3 * time.Second,
		subscribers:               make(map[uint64]chan managementevents.Frame),
	}
}
