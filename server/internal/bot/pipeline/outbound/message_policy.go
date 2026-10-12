package outbound

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// MessagePolicy resolves the sending identity before applying outbound limits.
type MessagePolicy struct {
	resolveScope func(chatevent.IdentityScope) (chatevent.IdentityScope, error)
	Limiter      *MessageRateLimiter
}

func NewMessagePolicy(cfg config.Config, resolveScope func(chatevent.IdentityScope) (chatevent.IdentityScope, error)) *MessagePolicy {
	return &MessagePolicy{resolveScope: resolveScope, Limiter: NewMessageRateLimiter(cfg)}
}

func (p *MessagePolicy) ApplyConfig(cfg config.Config) {
	if p.Limiter != nil {
		p.Limiter.ApplyConfig(cfg)
	}
}

func (p *MessagePolicy) Wait(ctx context.Context, request MessageLimitRequest) error {
	request, err := p.resolve(request)
	if err != nil {
		return err
	}
	return p.Limiter.Wait(ctx, request)
}

func (p *MessagePolicy) TryAdmit(request MessageLimitRequest) error {
	request, err := p.resolve(request)
	if err != nil {
		return err
	}
	return p.Limiter.TryAdmit(request)
}

func (p *MessagePolicy) resolve(request MessageLimitRequest) (MessageLimitRequest, error) {
	var err error
	if p.resolveScope != nil {
		request.Scope, err = p.resolveScope(request.Scope)
	}
	return request, err
}

// Begin binds quota admission to one identity snapshot, even if the adapter
// reconnects while the send is in flight.
func (p *MessagePolicy) Begin(ctx context.Context, request MessageLimitRequest) (MessageAdmission, error) {
	request, err := p.resolve(request)
	if err != nil {
		return MessageAdmission{Scope: request.Scope}, err
	}
	err = p.Limiter.Wait(ctx, request)
	return MessageAdmission{Scope: request.Scope}, err
}

// MessageAdmission carries the identity the send must use.
type MessageAdmission struct {
	Scope chatevent.IdentityScope
}
