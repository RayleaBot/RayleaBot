package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (m *Manager) Issue(subject string) (string, Claims, error) {
	return m.IssueWithContext(context.Background(), subject)
}

func (m *Manager) IssueWithContext(ctx context.Context, subject string) (string, Claims, error) {
	ctx = normalizeContext(ctx)
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", Claims{}, fmt.Errorf("subject is required")
	}

	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()
	return m.issueSerialized(ctx, subject, m.now().UTC())
}

func (m *Manager) Revoke(sessionID string) error {
	return m.RevokeWithContext(context.Background(), sessionID)
}

func (m *Manager) RevokeWithContext(ctx context.Context, sessionID string) error {
	ctx = normalizeContext(ctx)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrInvalidToken
	}

	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()

	m.stateMu.Lock()
	delete(m.sessions, sessionID)
	m.stateMu.Unlock()
	return m.deleteSessions(ctx, sessionID)
}

func (m *Manager) Validate(token string) (Claims, error) {
	return m.ValidateWithContext(context.Background(), token)
}

func (m *Manager) ValidateWithContext(ctx context.Context, token string) (Claims, error) {
	ctx = normalizeContext(ctx)
	token = strings.TrimSpace(token)
	if token == "" {
		return Claims{}, ErrInvalidToken
	}

	parsed, err := m.verify(token)
	if err != nil {
		return Claims{}, err
	}

	now := m.now().UTC()
	m.stateMu.RLock()
	stored, ok := m.sessions[parsed.SessionID]
	m.stateMu.RUnlock()
	if !ok || stored.Subject != parsed.Subject || !stored.IssuedAt.Equal(parsed.IssuedAt) {
		return Claims{}, ErrInvalidToken
	}

	absoluteExpiry := canonicalSessionTimestamp(stored.IssuedAt.Add(m.absoluteTTL()))
	if !now.Before(stored.ExpiresAt) || !now.Before(absoluteExpiry) {
		return Claims{}, m.expireSession(ctx, stored.SessionID, now)
	}

	if !m.cfg.SlidingRenewal || stored.ExpiresAt.Sub(now) > m.ttl()/2 {
		return stored, nil
	}

	return m.renewSession(ctx, stored.SessionID, now)
}

func (m *Manager) expireSession(ctx context.Context, sessionID string, now time.Time) error {
	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()

	m.stateMu.Lock()
	stored, ok := m.sessions[sessionID]
	if ok {
		absoluteExpiry := canonicalSessionTimestamp(stored.IssuedAt.Add(m.absoluteTTL()))
		if now.Before(stored.ExpiresAt) && now.Before(absoluteExpiry) {
			m.stateMu.Unlock()
			return ErrExpiredToken
		}
		delete(m.sessions, sessionID)
	}
	m.stateMu.Unlock()
	if ok {
		if err := m.deleteSessions(ctx, sessionID); err != nil {
			return err
		}
	}
	return ErrExpiredToken
}

func (m *Manager) renewSession(ctx context.Context, sessionID string, now time.Time) (Claims, error) {
	m.mutationMu.Lock()
	defer m.mutationMu.Unlock()

	m.stateMu.RLock()
	stored, ok := m.sessions[sessionID]
	m.stateMu.RUnlock()
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	absoluteExpiry := canonicalSessionTimestamp(stored.IssuedAt.Add(m.absoluteTTL()))
	if !now.Before(stored.ExpiresAt) || !now.Before(absoluteExpiry) {
		m.stateMu.Lock()
		delete(m.sessions, sessionID)
		m.stateMu.Unlock()
		if err := m.deleteSessions(ctx, sessionID); err != nil {
			return Claims{}, err
		}
		return Claims{}, ErrExpiredToken
	}
	if stored.ExpiresAt.Sub(now) > m.ttl()/2 {
		return stored, nil
	}

	renewedExpiry := canonicalSessionTimestamp(now.Add(m.ttl()))
	if renewedExpiry.After(absoluteExpiry) {
		renewedExpiry = absoluteExpiry
	}
	if !renewedExpiry.After(stored.ExpiresAt) {
		return stored, nil
	}
	stored.ExpiresAt = renewedExpiry
	if err := m.saveSession(ctx, stored); err != nil {
		return Claims{}, err
	}
	m.stateMu.Lock()
	m.sessions[sessionID] = stored
	m.stateMu.Unlock()
	return stored, nil
}

func (m *Manager) ttl() time.Duration {
	return time.Duration(m.cfg.SessionTTLDays) * 24 * time.Hour
}

func (m *Manager) absoluteTTL() time.Duration {
	return time.Duration(m.cfg.SessionAbsoluteTTLDays) * 24 * time.Hour
}

func canonicalSessionTimestamp(timestamp time.Time) time.Time {
	return timestamp.UTC().Truncate(time.Second)
}

func (m *Manager) issueSerialized(ctx context.Context, subject string, now time.Time) (string, Claims, error) {
	token, claims, err := m.newTokenClaims(subject, now)
	if err != nil {
		return "", Claims{}, err
	}

	m.stateMu.RLock()
	removed := m.sessionIDsToRemoveLocked(now)
	activeCount := len(m.sessions) - len(removed)
	m.stateMu.RUnlock()
	if activeCount >= m.cfg.MaxSessions {
		return "", Claims{}, ErrSessionLimitReached
	}
	if err := m.deleteSessions(ctx, removed...); err != nil {
		return "", Claims{}, err
	}
	if err := m.saveSession(ctx, claims); err != nil {
		return "", Claims{}, err
	}

	m.stateMu.Lock()
	for _, sessionID := range removed {
		delete(m.sessions, sessionID)
	}
	m.sessions[claims.SessionID] = claims
	m.stateMu.Unlock()
	return token, claims, nil
}

func (m *Manager) sessionIDsToRemoveLocked(now time.Time) []string {
	removed := make([]string, 0)
	remaining := make(map[string]Claims, len(m.sessions))
	for sessionID, claims := range m.sessions {
		absoluteExpiry := canonicalSessionTimestamp(claims.IssuedAt.Add(m.absoluteTTL()))
		if !now.Before(claims.ExpiresAt) || !now.Before(absoluteExpiry) {
			removed = append(removed, sessionID)
			continue
		}
		remaining[sessionID] = claims
	}
	for len(remaining) >= m.cfg.MaxSessions {
		sessionID, ok := oldestSessionID(remaining)
		if !ok {
			break
		}
		delete(remaining, sessionID)
		removed = append(removed, sessionID)
	}
	return removed
}

func oldestSessionID(sessions map[string]Claims) (string, bool) {
	var oldest Claims
	found := false
	for _, claims := range sessions {
		if !found || claims.IssuedAt.Before(oldest.IssuedAt) ||
			(claims.IssuedAt.Equal(oldest.IssuedAt) && claims.SessionID < oldest.SessionID) {
			oldest = claims
			found = true
		}
	}
	return oldest.SessionID, found
}
