package outbound

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

const (
	defaultMessageRateLimitPerPlugin = "20/10s"
	defaultMessageRateLimitPerTarget = "5/5s"
)

// MessageLimitRequest identifies one outbound message for platform throttling.
type MessageLimitRequest struct {
	Scope      chatevent.IdentityScope
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
	pluginLimiter *windowLimiter
	targetLimiter *windowLimiter
}

// NewMessageRateLimiter creates an outbound message limiter from user config.
func NewMessageRateLimiter(cfg config.Config) *MessageRateLimiter {
	limiter := &MessageRateLimiter{
		pluginLimiter: newWindowLimiter(time.Now, parseOutboundRateLimit(cfg.Message.RateLimitPerPlugin, defaultMessageRateLimitPerPlugin)),
		targetLimiter: newWindowLimiter(time.Now, parseOutboundRateLimit(cfg.Message.RateLimitPerTarget, defaultMessageRateLimitPerTarget)),
	}
	return limiter
}

// ApplyConfig refreshes limiter settings from the latest saved config.
func (l *MessageRateLimiter) ApplyConfig(cfg config.Config) {

	pluginLimit := parseOutboundRateLimit(cfg.Message.RateLimitPerPlugin, defaultMessageRateLimitPerPlugin)
	targetLimit := parseOutboundRateLimit(cfg.Message.RateLimitPerTarget, defaultMessageRateLimitPerTarget)
	l.pluginLimiter.SetLimit(pluginLimit)
	l.targetLimiter.SetLimit(targetLimit)
}

// Wait blocks in FIFO order until the message can be sent or the configured
// wait limit is reached.
func (l *MessageRateLimiter) Wait(ctx context.Context, request MessageLimitRequest) error {

	pluginID := strings.TrimSpace(request.PluginID)
	if pluginID != "" {
		if err := l.pluginLimiter.Wait(ctx, "plugin:"+pluginID); err != nil {
			return rateLimitedError()
		}
	}

	targetType := strings.TrimSpace(request.TargetType)
	targetID := strings.TrimSpace(request.TargetID)
	if targetType != "" && targetID != "" {
		if err := l.targetLimiter.Wait(ctx, "target:"+request.Scope.Key(targetType, targetID)); err != nil {
			return rateLimitedError()
		}
	}

	return nil
}

func rateLimitedError() error {
	return &onebot11.Error{
		Code:    errorcodes.PlatformRateLimited,
		Message: "outbound message rate limit exceeded",
	}
}

func parseOutboundRateLimit(raw string, fallback string) config.RateLimit {
	limit, err := config.ParseRateLimit(strings.TrimSpace(raw))
	if err == nil {
		return limit
	}
	limit, _ = config.ParseRateLimit(fallback)
	return limit
}
