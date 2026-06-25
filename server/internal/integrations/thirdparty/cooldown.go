package thirdparty

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/fingerprint"
)

const (
	DefaultCooldownBase = 5 * time.Second
	DefaultCooldownMax  = 30 * time.Minute
)

type cooldownEntry struct {
	Until     time.Time
	Attempts  int
	Scope     string
	LastError string
}

type CooldownManager struct {
	mu        sync.Mutex
	cooldowns map[string]cooldownEntry
	identity  *fingerprint.IdentityProvider
	baseDelay time.Duration
	maxDelay  time.Duration
	now       func() time.Time
}

func NewCooldownManager(identity *fingerprint.IdentityProvider, now func() time.Time) *CooldownManager {
	if now == nil {
		now = time.Now
	}
	baseDelay := DefaultCooldownBase
	maxDelay := DefaultCooldownMax
	return &CooldownManager{
		cooldowns: make(map[string]cooldownEntry),
		identity:  identity,
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
		now:       now,
	}
}

func (m *CooldownManager) WithDelays(base, max time.Duration) *CooldownManager {
	if base > 0 {
		m.baseDelay = base
	}
	if max > 0 {
		m.maxDelay = max
	}
	return m
}

func (m *CooldownManager) ShouldWait(key string) time.Duration {
	if key == "" {
		return 0
	}
	key = strings.TrimSpace(key)
	m.mu.Lock()
	cooldown := m.cooldowns[key]
	m.mu.Unlock()
	now := m.now()
	if cooldown.Until.IsZero() || !now.Before(cooldown.Until) {
		return 0
	}
	return cooldown.Until.Sub(now)
}

func (m *CooldownManager) RecordError(key string, err error) {
	if key == "" || err == nil {
		return
	}
	key = strings.TrimSpace(key)
	now := m.now()
	m.mu.Lock()
	cooldown := m.cooldowns[key]
	cooldown.Attempts++
	delay := m.baseDelay
	for i := 1; i < cooldown.Attempts; i++ {
		delay *= 2
		if delay >= m.maxDelay {
			delay = m.maxDelay
			break
		}
	}
	if m.identity != nil {
		delay = m.identity.JitteredDelay(delay)
	}
	cooldown.Until = now.Add(delay)
	cooldown.LastError = err.Error()
	m.cooldowns[key] = cooldown
	m.mu.Unlock()
}

func (m *CooldownManager) Clear(key string) {
	if key == "" {
		return
	}
	key = strings.TrimSpace(key)
	m.mu.Lock()
	delete(m.cooldowns, key)
	m.mu.Unlock()
}

func (m *CooldownManager) Attempts(key string) int {
	key = strings.TrimSpace(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cooldowns[key].Attempts
}

func RecordCooldownError(mgr *CooldownManager, platform, accountID string, err error) {
	if mgr == nil || !IsRequestCooldownError(err) {
		return
	}
	key := fmt.Sprintf("%s:%s", strings.TrimSpace(platform), strings.TrimSpace(accountID))
	mgr.RecordError(key, err)
}
