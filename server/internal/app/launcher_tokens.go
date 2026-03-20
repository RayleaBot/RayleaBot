package app

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
	"time"
)

type launcherTokenStore struct {
	mu     sync.Mutex
	now    func() time.Time
	ttl    time.Duration
	tokens map[string]time.Time
}

func newLauncherTokenStore(now func() time.Time, ttl time.Duration) *launcherTokenStore {
	if now == nil {
		now = time.Now
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	return &launcherTokenStore{
		now:    now,
		ttl:    ttl,
		tokens: make(map[string]time.Time),
	}
}

func (s *launcherTokenStore) Issue() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneExpiredLocked()

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	token := "launcher_" + base64.RawURLEncoding.EncodeToString(buf)
	s.tokens[token] = s.now().UTC().Add(s.ttl)
	return token, nil
}

func (s *launcherTokenStore) Consume(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneExpiredLocked()
	expiresAt, ok := s.tokens[token]
	if !ok || !s.now().UTC().Before(expiresAt) {
		delete(s.tokens, token)
		return false
	}

	delete(s.tokens, token)
	return true
}

func (s *launcherTokenStore) pruneExpiredLocked() {
	now := s.now().UTC()
	for token, expiresAt := range s.tokens {
		if !now.Before(expiresAt) {
			delete(s.tokens, token)
		}
	}
}
