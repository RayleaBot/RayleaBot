package thirdparty

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/fingerprint"
)

type ErrorKind string

const (
	ErrorAuth            ErrorKind = "auth"
	ErrorCSRF            ErrorKind = "csrf"
	ErrorRiskControl     ErrorKind = "risk_control"
	ErrorCaptcha         ErrorKind = "captcha"
	ErrorRateLimit       ErrorKind = "rate_limit"
	ErrorSignature       ErrorKind = "signature"
	ErrorNotFound        ErrorKind = "not_found"
	ErrorBadRequest      ErrorKind = "bad_request"
	ErrorServer          ErrorKind = "server"
	ErrorInvalidResponse ErrorKind = "invalid_response"
	ErrorUpstream        ErrorKind = "upstream"
	ErrorNetwork         ErrorKind = "network"
	ErrorExpired         ErrorKind = "expired"
)

type ThirdPartyError struct {
	Platform   string
	Kind       ErrorKind
	Code       int
	HTTPStatus int
	Message    string
	Body       string
	Err        error
}

func (e *ThirdPartyError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{e.Platform, string(e.Kind)}
	if e.Code != 0 {
		parts = append(parts, "code "+strconv.Itoa(e.Code))
	}
	if e.HTTPStatus != 0 {
		parts = append(parts, "HTTP "+strconv.Itoa(e.HTTPStatus))
	}
	if strings.TrimSpace(e.Message) != "" {
		parts = append(parts, strings.TrimSpace(e.Message))
	}
	if e.Err != nil && strings.TrimSpace(e.Err.Error()) != "" {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *ThirdPartyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func AsThirdPartyError(err error) *ThirdPartyError {
	var target *ThirdPartyError
	if errors.As(err, &target) {
		return target
	}
	return nil
}

func IsRiskControlError(err error) bool {
	tpErr := AsThirdPartyError(err)
	return tpErr != nil && tpErr.Kind == ErrorRiskControl
}

func IsRateLimitError(err error) bool {
	tpErr := AsThirdPartyError(err)
	return tpErr != nil && tpErr.Kind == ErrorRateLimit
}

func IsRequestCooldownError(err error) bool {
	tpErr := AsThirdPartyError(err)
	if tpErr == nil {
		return false
	}
	return tpErr.Kind == ErrorRiskControl || tpErr.Kind == ErrorRateLimit
}

func ClassifyHTTPStatus(status int) ErrorKind {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorAuth
	case http.StatusBadRequest:
		return ErrorBadRequest
	case http.StatusNotFound:
		return ErrorNotFound
	case http.StatusTooManyRequests:
		return ErrorRateLimit
	default:
		if status >= 500 {
			return ErrorServer
		}
		if status >= 400 {
			return ErrorUpstream
		}
		return ErrorUpstream
	}
}

func NewPlatformError(platform string, kind ErrorKind, code int, httpStatus int, message string, err error) *ThirdPartyError {
	return &ThirdPartyError{
		Platform:   platform,
		Kind:       kind,
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    strings.TrimSpace(message),
		Err:        err,
	}
}

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
