package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (m *Manager) Issue(subject string) (string, Claims, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", Claims{}, fmt.Errorf("subject is required")
	}

	now := m.now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.issueLocked(subject, now)
}

func (m *Manager) Revoke(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrInvalidToken
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return m.deleteSessionsLocked(context.Background(), sessionID)
}

func (m *Manager) Validate(token string) (Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Claims{}, ErrInvalidToken
	}

	parsed, err := m.verify(token)
	if err != nil {
		return Claims{}, err
	}

	now := m.now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	stored, ok := m.sessions[parsed.SessionID]
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	if stored.Subject != parsed.Subject || !stored.IssuedAt.Equal(parsed.IssuedAt) {
		return Claims{}, ErrInvalidToken
	}
	if !now.Before(stored.ExpiresAt) {
		delete(m.sessions, stored.SessionID)
		if err := m.deleteSessionsLocked(context.Background(), stored.SessionID); err != nil {
			return Claims{}, err
		}
		return Claims{}, ErrExpiredToken
	}

	if m.cfg.SlidingRenewal {
		stored.ExpiresAt = canonicalSessionTimestamp(now.Add(m.ttl()))
		if err := m.saveSessionLocked(context.Background(), stored); err != nil {
			return Claims{}, err
		}
		m.sessions[stored.SessionID] = stored
	}

	return stored, nil
}

func (m *Manager) ttl() time.Duration {
	return time.Duration(m.cfg.SessionTTLDays) * 24 * time.Hour
}

func canonicalSessionTimestamp(timestamp time.Time) time.Time {
	return timestamp.UTC().Truncate(time.Second)
}

func (m *Manager) pruneExpiredLocked(now time.Time) []string {
	var removed []string
	for sessionID, claims := range m.sessions {
		if !now.Before(claims.ExpiresAt) {
			delete(m.sessions, sessionID)
			removed = append(removed, sessionID)
		}
	}
	return removed
}

func (m *Manager) recycleOldestSessionsLocked() []string {
	if m.cfg.MaxSessions <= 0 {
		return nil
	}

	var removed []string
	for len(m.sessions) >= m.cfg.MaxSessions {
		sessionID, ok := m.oldestSessionIDLocked()
		if !ok {
			break
		}

		delete(m.sessions, sessionID)
		removed = append(removed, sessionID)
	}

	return removed
}

func (m *Manager) oldestSessionIDLocked() (string, bool) {
	var oldest Claims
	found := false
	for _, claims := range m.sessions {
		if !found ||
			claims.IssuedAt.Before(oldest.IssuedAt) ||
			(claims.IssuedAt.Equal(oldest.IssuedAt) && claims.SessionID < oldest.SessionID) {
			oldest = claims
			found = true
		}
	}

	if !found {
		return "", false
	}

	return oldest.SessionID, true
}

func (m *Manager) issueLocked(subject string, now time.Time) (string, Claims, error) {
	token, claims, err := m.newTokenClaimsLocked(subject, now)
	if err != nil {
		return "", Claims{}, err
	}

	removed := m.pruneExpiredLocked(now)
	removed = append(removed, m.recycleOldestSessionsLocked()...)
	if err := m.deleteSessionsLocked(context.Background(), removed...); err != nil {
		return "", Claims{}, err
	}
	if len(m.sessions) >= m.cfg.MaxSessions {
		return "", Claims{}, ErrSessionLimitReached
	}
	if err := m.saveSessionLocked(context.Background(), claims); err != nil {
		return "", Claims{}, err
	}

	m.sessions[claims.SessionID] = claims
	return token, claims, nil
}
