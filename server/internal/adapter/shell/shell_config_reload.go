package shell

import (
	"net/http"
	"time"

	adaptercache "github.com/RayleaBot/RayleaBot/server/internal/adapter/cache"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func (s *Shell) applyConfig(nextCfg config.OneBotConfig, nextAdapterCfg config.AdapterConfig, previousCfg config.OneBotConfig, previousAdapterCfg config.AdapterConfig) {
	s.mu.Lock()
	s.cfg = nextCfg
	s.adapterCfg = nextAdapterCfg
	s.deps.connectTimeout = nextConnectTimeout(previousAdapterCfg, nextAdapterCfg, s.deps.connectTimeout)
	s.deps.backoff = nextBackoff(previousAdapterCfg, nextAdapterCfg, s.deps.backoff)
	s.httpClient = &http.Client{
		Timeout: s.deps.connectTimeout,
	}
	s.snapshot = newTransportSnapshot(nextCfg)
	s.pendingResponses = make(map[string]chan adapteroutbound.APIResponse)
	s.recentEventIDs = make(map[string]time.Time)
	s.identityCache = adaptercache.NewIdentityCache(defaultIdentityCacheTTL)
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()

	s.emitStateSnapshot(handler, snapshot)
}

func nextConnectTimeout(previousCfg config.AdapterConfig, nextCfg config.AdapterConfig, current time.Duration) time.Duration {
	if nextCfg.ConnectTimeoutSeconds == previousCfg.ConnectTimeoutSeconds && current > 0 {
		return current
	}

	return time.Duration(maxInt(nextCfg.ConnectTimeoutSeconds, 1)) * time.Second
}

func nextBackoff(previousCfg config.AdapterConfig, nextCfg config.AdapterConfig, current *Backoff) *Backoff {
	if reconnectSettingsEqual(previousCfg, nextCfg) && current != nil {
		return current
	}

	var randFloat func() float64
	if current != nil {
		randFloat = current.randFloat
	}

	return NewBackoff(
		nextCfg.ReconnectInitialSeconds,
		nextCfg.ReconnectMultiplier,
		nextCfg.ReconnectMaxSeconds,
		nextCfg.ReconnectJitterRatio,
		randFloat,
	)
}

func reconnectSettingsEqual(left config.AdapterConfig, right config.AdapterConfig) bool {
	return left.ReconnectInitialSeconds == right.ReconnectInitialSeconds &&
		left.ReconnectMultiplier == right.ReconnectMultiplier &&
		left.ReconnectMaxSeconds == right.ReconnectMaxSeconds &&
		left.ReconnectJitterRatio == right.ReconnectJitterRatio
}
