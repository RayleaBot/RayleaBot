package source

import (
	"strings"
	"time"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func isBilibiliRequestCooldownError(err error) bool {
	biliErr := bilibiliSession.AsError(err)
	if biliErr == nil {
		return false
	}
	return biliErr.Kind == bilibiliSession.ErrorRiskControl || biliErr.Kind == bilibiliSession.ErrorRateLimit
}

func (s *Source) requestCooldownDelay(scope string, account thirdparty.Account, cookie string) time.Duration {
	key := requestCooldownKey(scope, account, cookie)
	if key == "" {
		return 0
	}
	now := s.now()
	s.mu.RLock()
	cooldown := s.cooldowns[key]
	s.mu.RUnlock()
	if cooldown.Until.IsZero() || !now.Before(cooldown.Until) {
		return 0
	}
	return cooldown.Until.Sub(now)
}

func (s *Source) rememberRequestCooldown(scope string, account thirdparty.Account, cookie string, err error) {
	key := requestCooldownKey(scope, account, cookie)
	if key == "" || err == nil {
		return
	}
	now := s.now()
	s.mu.Lock()
	cooldown := s.cooldowns[key]
	cooldown.Attempts++
	cooldown.Scope = normalizeCooldownScope(scope)
	cooldown.Code = cooldownCode(err)
	delay := bilibiliRiskControlCooldownBase
	for i := 1; i < cooldown.Attempts; i++ {
		delay *= 2
		if delay >= bilibiliRiskControlCooldownMax {
			delay = bilibiliRiskControlCooldownMax
			break
		}
	}
	delay = s.identity.JitteredDelay(delay)
	cooldown.Until = now.Add(delay)
	cooldown.LastError = err.Error()
	s.cooldowns[key] = cooldown
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}

func (s *Source) clearRequestCooldown(scope string, account thirdparty.Account, cookie string) {
	key := requestCooldownKey(scope, account, cookie)
	if key == "" {
		return
	}
	s.mu.Lock()
	delete(s.cooldowns, key)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}

func requestCooldownKey(scope string, account thirdparty.Account, cookie string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return ""
	}
	accountKey := strings.TrimSpace(account.Platform + ":" + account.AccountID)
	if accountKey == ":" {
		accountKey = cookieFingerprint(cookie)
	}
	if accountKey == "" {
		return ""
	}
	return scope + ":" + accountKey + ":" + cookieFingerprint(cookie)
}
