package outbound

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
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
	if l == nil {
		return
	}

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
	if l == nil {
		return nil
	}

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
