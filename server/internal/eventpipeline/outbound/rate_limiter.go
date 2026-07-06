package outbound

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

const (
	defaultMessageRateLimitPerPlugin = "20/10s"
	defaultMessageRateLimitPerTarget = "5/5s"
	defaultMessageCircuitBreakerSecs = 30
)

// MessageLimitRequest identifies one outbound message for platform throttling.
type MessageLimitRequest struct {
	PluginID   string
	TargetType string
	TargetID   string
}

// MessageLimiter waits until an outbound message is allowed to leave.
type MessageLimiter interface {
	Wait(context.Context, MessageLimitRequest) error
}

// MessageRateLimiter enforces plugin and target outbound message limits.
type MessageRateLimiter struct {
	mu            sync.RWMutex
	maxWait       time.Duration
	pluginLimiter *windowLimiter
	targetLimiter *windowLimiter
}

// NewMessageRateLimiter creates an outbound message limiter from user config.
func NewMessageRateLimiter(cfg config.Config) *MessageRateLimiter {
	limiter := &MessageRateLimiter{
		pluginLimiter: newWindowLimiter(time.Now, parseOutboundRateLimit(cfg.Message.RateLimitPerPlugin, defaultMessageRateLimitPerPlugin)),
		targetLimiter: newWindowLimiter(time.Now, parseOutboundRateLimit(cfg.Message.RateLimitPerTarget, defaultMessageRateLimitPerTarget)),
		maxWait:       messageCircuitBreaker(cfg),
	}
	return limiter
}

// ApplyConfig refreshes limiter settings from the latest saved config.
func (l *MessageRateLimiter) ApplyConfig(cfg config.Config) {

	pluginLimit := parseOutboundRateLimit(cfg.Message.RateLimitPerPlugin, defaultMessageRateLimitPerPlugin)
	targetLimit := parseOutboundRateLimit(cfg.Message.RateLimitPerTarget, defaultMessageRateLimitPerTarget)
	maxWait := messageCircuitBreaker(cfg)

	l.mu.Lock()
	l.maxWait = maxWait
	l.mu.Unlock()

	l.pluginLimiter.SetLimit(pluginLimit)
	l.targetLimiter.SetLimit(targetLimit)
}

// Wait blocks in FIFO order until the message can be sent or the configured
// wait limit is reached.
func (l *MessageRateLimiter) Wait(ctx context.Context, request MessageLimitRequest) error {

	l.mu.RLock()
	maxWait := l.maxWait
	l.mu.RUnlock()

	if maxWait > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, maxWait)
		defer cancel()
	}

	pluginID := strings.TrimSpace(request.PluginID)
	if pluginID != "" {
		if err := l.pluginLimiter.Wait(ctx, "plugin:"+pluginID); err != nil {
			return rateLimitedError()
		}
	}

	targetType := strings.TrimSpace(request.TargetType)
	targetID := strings.TrimSpace(request.TargetID)
	if targetType != "" && targetID != "" {
		if err := l.targetLimiter.Wait(ctx, "target:"+targetType+":"+targetID); err != nil {
			return rateLimitedError()
		}
	}

	return nil
}

func rateLimitedError() error {
	return &onebot11.Error{
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
