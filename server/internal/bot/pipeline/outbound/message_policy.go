package outbound

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// MessagePolicy resolves the sending identity before applying outbound limits.
type MessagePolicy struct {
	resolveScope func(chatevent.IdentityScope) chatevent.IdentityScope
	Limiter      *MessageRateLimiter
}

func NewMessagePolicy(cfg config.Config, resolveScope func(chatevent.IdentityScope) chatevent.IdentityScope) *MessagePolicy {
	return &MessagePolicy{resolveScope: resolveScope, Limiter: NewMessageRateLimiter(cfg)}
}

func (p *MessagePolicy) ApplyConfig(cfg config.Config) {
	if p.Limiter != nil {
		p.Limiter.ApplyConfig(cfg)
	}
}

func (p *MessagePolicy) Wait(ctx context.Context, request MessageLimitRequest) error {
	return p.Limiter.Wait(ctx, p.resolve(request))
}

func (p *MessagePolicy) TryAdmit(request MessageLimitRequest) error {
	return p.Limiter.TryAdmit(p.resolve(request))
}

func (p *MessagePolicy) resolve(request MessageLimitRequest) MessageLimitRequest {
	if p.resolveScope != nil {
		request.Scope = p.resolveScope(request.Scope)
	}
	return request
}

// Begin binds quota admission to one identity snapshot, even if the adapter
// reconnects while the send is in flight.
func (p *MessagePolicy) Begin(ctx context.Context, request MessageLimitRequest) (MessageAdmission, error) {
	request = p.resolve(request)
	err := p.Limiter.Wait(ctx, request)
	return MessageAdmission{Scope: request.Scope}, err
}

// MessageAdmission carries the identity the send must use.
type MessageAdmission struct {
	Scope chatevent.IdentityScope
}
