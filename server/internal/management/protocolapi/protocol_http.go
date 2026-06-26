package protocolapi

import (
	"sync"
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
)

type ConfigSource interface {
	CurrentConfig() config.Config
}

type ProtocolService struct {
	config                    ConfigSource
	adapter                   *adaptershell.Shell
	oneBot11TargetReadTimeout time.Duration
	mu                        sync.RWMutex
	nextSubID                 uint64
	subscribers               map[uint64]chan managementevents.Frame
}

func NewProtocolService(configSource ConfigSource, adapterShell *adaptershell.Shell) *ProtocolService {
	return &ProtocolService{
		config:                    configSource,
		adapter:                   adapterShell,
		oneBot11TargetReadTimeout: 3 * time.Second,
		subscribers:               make(map[uint64]chan managementevents.Frame),
	}
}
