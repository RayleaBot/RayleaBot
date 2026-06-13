package dispatch

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (d *Dispatcher) waitOutboundLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	limiter := d.outboundLimiter
	d.mu.RUnlock()
	if limiter == nil {
		return nil
	}
	return limiter.Wait(ctx, request)
}

func (d *Dispatcher) limitTargetForAction(action runtime.Action) (string, string) {
	if action.Kind == "message.reply" && d != nil && d.resolver != nil {
		if target, ok := d.resolver.ResolveReplyTarget(strings.TrimSpace(action.ReplyToEventID)); ok {
			return target.TargetType, target.TargetID
		}
	}
	return action.TargetType, action.TargetID
}
