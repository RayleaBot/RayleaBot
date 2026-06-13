package app

import (
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
)

type protocolService struct {
	state       *appRuntimeState
	adapter     *adapter.Shell
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan managementEventFrame
}

func newProtocolService(state *appRuntimeState, adapterShell *adapter.Shell) *protocolService {
	return &protocolService{
		state:       state,
		adapter:     adapterShell,
		subscribers: make(map[uint64]chan managementEventFrame),
	}
}
