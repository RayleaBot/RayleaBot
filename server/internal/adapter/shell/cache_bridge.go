package shell

import (
	"time"

	adaptercache "github.com/RayleaBot/RayleaBot/server/internal/adapter/cache"
)

type IdentityCache = adaptercache.IdentityCache

func NewIdentityCache(ttl time.Duration) *IdentityCache {
	return adaptercache.NewIdentityCache(ttl)
}
