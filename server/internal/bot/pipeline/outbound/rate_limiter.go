package outbound

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

const defaultMessageRateLimitPerTarget = "5/5s"

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

// MessageRateLimiter enforces the per-target outbound message limit. Platform
// risk control reacts to one conversation being flooded, so the limit is keyed
// by the resolved bot identity and conversation.
type MessageRateLimiter struct {
	targetLimiter *windowLimiter
}

// NewMessageRateLimiter creates an outbound message limiter from user config.
func NewMessageRateLimiter(cfg config.Config) *MessageRateLimiter {
	return &MessageRateLimiter{
		targetLimiter: newWindowLimiter(time.Now, parseOutboundRateLimit(cfg.Message.RateLimitPerTarget, defaultMessageRateLimitPerTarget)),
	}
}

// ApplyConfig refreshes limiter settings from the latest saved config.
func (l *MessageRateLimiter) ApplyConfig(cfg config.Config) {
	l.targetLimiter.SetLimit(parseOutboundRateLimit(cfg.Message.RateLimitPerTarget, defaultMessageRateLimitPerTarget))
}

// Wait blocks in FIFO order until the message can be sent or the context ends.
func (l *MessageRateLimiter) Wait(ctx context.Context, request MessageLimitRequest) error {
	targetType := strings.TrimSpace(request.TargetType)
	targetID := strings.TrimSpace(request.TargetID)
	if targetType == "" || targetID == "" {
		return nil
	}
	if err := l.targetLimiter.Wait(ctx, "target:"+request.Scope.Key(targetType, targetID)); err != nil {
		return rateLimitedError()
	}
	return nil
}

func rateLimitedError() error {
	return &chatevent.SendError{
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
