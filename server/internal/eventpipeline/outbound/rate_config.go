package outbound

import (
	"strings"
	"time"

	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

func rateLimitedError() error {
	return &adapteroutbound.Error{
		Code:    "platform.rate_limited",
		Message: "outbound message rate limit exceeded",
	}
}

func parseOutboundRateLimit(raw string, fallback string) permission.RateLimit {
	limit, err := permission.ParseRateLimit(strings.TrimSpace(raw))
	if err == nil {
		return limit
	}
	limit, _ = permission.ParseRateLimit(fallback)
	return limit
}

func messageCircuitBreaker(cfg config.Config) time.Duration {
	seconds := cfg.Message.CircuitBreakerSeconds
	if seconds <= 0 {
		seconds = defaultMessageCircuitBreakerSecs
	}
	return time.Duration(seconds) * time.Second
}
