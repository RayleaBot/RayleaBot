package management

import (
	"crypto/hmac"
	"sync"
)

const (
	SessionCookieName          = "raylea_session"
	SessionTransportHeader     = "X-Raylea-Session-Transport"
	CSRFHeader                 = "X-Raylea-CSRF"
	SetupTokenHeader           = "X-Raylea-Setup-Token"
	LauncherControlTokenHeader = "X-Raylea-Launcher-Control"
)

type OneTimeToken struct {
	mu       sync.Mutex
	value    []byte
	consumed bool
}

func NewOneTimeToken(value string) *OneTimeToken {
	return &OneTimeToken{value: []byte(value)}
}

func (t *OneTimeToken) Consume(candidate string) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.consumed || len(t.value) == 0 || !hmac.Equal(t.value, []byte(candidate)) {
		return false
	}
	t.consumed = true
	for index := range t.value {
		t.value[index] = 0
	}
	return true
}

type StaticToken struct {
	value []byte
}

func NewStaticToken(value string) *StaticToken {
	return &StaticToken{value: []byte(value)}
}

func (t *StaticToken) Matches(candidate string) bool {
	return t != nil && len(t.value) > 0 && hmac.Equal(t.value, []byte(candidate))
}
