package outbound

import (
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

type windowLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   permission.RateLimit
	updated chan struct{}
	windows map[string]*windowState
}

type windowState struct {
	queue   []*windowWaiter
	records []time.Time
}

type windowWaiter struct {
	ready chan struct{}
}
