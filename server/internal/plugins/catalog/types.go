package catalog

import (
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type Catalog struct {
	mu          sync.RWMutex
	order       []string
	items       map[string]plugins.Snapshot
	nextSubID   uint64
	subscribers map[uint64]chan plugins.Snapshot
}
