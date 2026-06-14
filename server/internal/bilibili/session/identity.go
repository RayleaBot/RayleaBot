package session

import (
	"math/rand/v2"
	"sync"
	"time"
)

type IdentityProvider struct {
	uaPool    []uaEntry
	languages []string
	now       func() time.Time
	mu        sync.Mutex
	uaIndex   int
	langIndex int
	rng       *rand.Rand
	fixedUA   string
}

func NewIdentityProvider(now func() time.Time) *IdentityProvider {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &IdentityProvider{
		uaPool:    defaultUAPool,
		languages: defaultLanguages,
		now:       now,
		rng:       rand.New(rand.NewPCG(uint64(now().UnixNano()), 0)),
	}
}

func (p *IdentityProvider) WithFixedUA(ua string) *IdentityProvider {
	p.mu.Lock()
	p.fixedUA = ua
	p.mu.Unlock()
	return p
}

func (p *IdentityProvider) UserAgent() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fixedUA != "" {
		return p.fixedUA
	}
	entry := p.uaPool[p.uaIndex%len(p.uaPool)]
	p.uaIndex++
	return entry.UA
}

func (p *IdentityProvider) currentEntry() uaEntry {
	p.mu.Lock()
	idx := p.uaIndex % len(p.uaPool)
	p.mu.Unlock()
	return p.uaPool[idx]
}

func (p *IdentityProvider) acceptLanguage() string {
	p.mu.Lock()
	lang := p.languages[p.langIndex%len(p.languages)]
	p.langIndex++
	p.mu.Unlock()
	return lang
}

func (p *IdentityProvider) JitteredDelay(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	p.mu.Lock()
	factor := 0.7 + p.rng.Float64()*0.6
	p.mu.Unlock()
	return time.Duration(float64(base) * factor)
}

func (p *IdentityProvider) formatCooldownDelay(delay time.Duration) string {
	if delay <= 0 {
		return "0s"
	}
	return delay.Round(time.Second).String()
}
