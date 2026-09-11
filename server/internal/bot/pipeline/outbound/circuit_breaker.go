package outbound

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

const (
	defaultMessageCircuitBreakerSecs = 30
	messageCircuitFailureThreshold   = 3
)

type circuitState struct {
	failures      int
	openUntil     time.Time
	probeInFlight bool
}

type MessageCircuitBreaker struct {
	mu        sync.Mutex
	now       func() time.Time
	cooldown  time.Duration
	threshold int
	states    map[string]*circuitState
}

func NewMessageCircuitBreaker(cfg config.Config) *MessageCircuitBreaker {
	return newMessageCircuitBreaker(time.Now, messageCircuitCooldown(cfg), messageCircuitFailureThreshold)
}

func newMessageCircuitBreaker(now func() time.Time, cooldown time.Duration, threshold int) *MessageCircuitBreaker {
	if now == nil {
		now = time.Now
	}
	if cooldown <= 0 {
		cooldown = defaultMessageCircuitBreakerSecs * time.Second
	}
	if threshold <= 0 {
		threshold = messageCircuitFailureThreshold
	}
	return &MessageCircuitBreaker{
		now:       now,
		cooldown:  cooldown,
		threshold: threshold,
		states:    make(map[string]*circuitState),
	}
}

func (b *MessageCircuitBreaker) ApplyConfig(cfg config.Config) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cooldown = messageCircuitCooldown(cfg)
}

func (b *MessageCircuitBreaker) Allow(request MessageLimitRequest) error {
	key := circuitKey(request)
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.states[key]
	if state == nil || state.openUntil.IsZero() {
		return nil
	}
	now := b.now().UTC()
	if now.Before(state.openUntil) || state.probeInFlight {
		return circuitOpenError()
	}
	state.probeInFlight = true
	return nil
}

func (b *MessageCircuitBreaker) Record(request MessageLimitRequest, sendErr error) {
	key := circuitKey(request)
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.states[key]
	if ignoreCircuitResult(sendErr) {
		if state != nil {
			state.probeInFlight = false
		}
		return
	}
	if sendErr == nil {
		delete(b.states, key)
		return
	}
	if state == nil {
		state = &circuitState{}
		b.states[key] = state
	}
	if state.probeInFlight || !state.openUntil.IsZero() {
		state.failures = b.threshold
		state.probeInFlight = false
		state.openUntil = b.now().UTC().Add(b.cooldown)
		return
	}
	state.failures++
	if state.failures >= b.threshold {
		state.openUntil = b.now().UTC().Add(b.cooldown)
	}
}

func ignoreCircuitResult(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var adapterErr *chatevent.SendError
	return errors.As(err, &adapterErr) && adapterErr.Code == errorcodes.AdapterReplyTargetMissing
}

func circuitKey(request MessageLimitRequest) string {
	targetType := strings.TrimSpace(request.TargetType)
	targetID := strings.TrimSpace(request.TargetID)
	if targetType != "" && targetID != "" {
		return "target:" + request.Scope.Key(targetType, targetID)
	}
	if pluginID := strings.TrimSpace(request.PluginID); pluginID != "" {
		return "plugin:" + pluginID
	}
	return "adapter:" + request.Scope.Key("", "")
}

func circuitOpenError() error {
	return &chatevent.SendError{
		Code:    errorcodes.AdapterSendFailed,
		Message: "outbound message circuit breaker is open",
	}
}

func messageCircuitCooldown(cfg config.Config) time.Duration {
	seconds := cfg.Message.CircuitBreakerSeconds
	if seconds <= 0 {
		seconds = defaultMessageCircuitBreakerSecs
	}
	return time.Duration(seconds) * time.Second
}

type MessagePolicy struct {
	resolveScope func(chatevent.IdentityScope) chatevent.IdentityScope
	Limiter      *MessageRateLimiter
	Breaker      *MessageCircuitBreaker
}

func NewMessagePolicy(cfg config.Config, resolveScope func(chatevent.IdentityScope) chatevent.IdentityScope) *MessagePolicy {
	policy := &MessagePolicy{
		resolveScope: resolveScope,
		Limiter:      NewMessageRateLimiter(cfg),
		Breaker:      NewMessageCircuitBreaker(cfg),
	}
	return policy
}

func (p *MessagePolicy) ApplyConfig(cfg config.Config) {
	if p.Limiter != nil {
		p.Limiter.ApplyConfig(cfg)
	}
	if p.Breaker != nil {
		p.Breaker.ApplyConfig(cfg)
	}
}

func (p *MessagePolicy) Wait(ctx context.Context, request MessageLimitRequest) error {
	return p.Limiter.Wait(ctx, p.resolve(request))
}

func (p *MessagePolicy) resolve(request MessageLimitRequest) MessageLimitRequest {
	if p.resolveScope != nil {
		request.Scope = p.resolveScope(request.Scope)
	}
	return request
}

// Begin binds quota admission and the eventual circuit result to one identity
// snapshot, even if the adapter reconnects while the send is in flight.
func (p *MessagePolicy) Begin(ctx context.Context, request MessageLimitRequest) (MessageAdmission, error) {
	request = p.resolve(request)
	if err := p.Limiter.Wait(ctx, request); err != nil {
		return MessageAdmission{Scope: request.Scope}, err
	}
	if err := p.Breaker.Allow(request); err != nil {
		return MessageAdmission{Scope: request.Scope}, err
	}
	return MessageAdmission{Scope: request.Scope, Record: func(err error) { p.Breaker.Record(request, err) }}, nil
}

type MessageAdmission struct {
	Scope  chatevent.IdentityScope
	Record func(error)
}
