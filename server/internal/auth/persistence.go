package auth

import (
	"context"
	"fmt"
)

func (m *Manager) hydrate(ctx context.Context) error {
	state, err := m.repo.LoadBootstrap(ctx)
	if err != nil {
		return fmt.Errorf("load bootstrap state: %w", err)
	}
	if state != nil {
		m.bootstrap = &bootstrapCredentials{
			Identifier:    state.Identifier,
			SecretDigest:  append([]byte(nil), state.SecretDigest...),
			InitializedAt: state.InitializedAt.UTC(),
		}
		m.signingKey = append([]byte(nil), state.SigningKey...)
	}

	sessions, err := m.repo.LoadSessions(ctx)
	if err != nil {
		return fmt.Errorf("load admin sessions: %w", err)
	}

	now := m.now().UTC()
	var expired []string
	for _, claims := range sessions {
		claims.IssuedAt = canonicalSessionTimestamp(claims.IssuedAt)
		claims.ExpiresAt = canonicalSessionTimestamp(claims.ExpiresAt)
		if now.Before(claims.ExpiresAt) {
			m.sessions[claims.SessionID] = claims
			continue
		}
		expired = append(expired, claims.SessionID)
	}
	if err := m.deleteSessionsLocked(ctx, expired...); err != nil {
		return err
	}

	return nil
}

func (m *Manager) saveSessionLocked(ctx context.Context, claims Claims) error {
	if m.repo == nil {
		return nil
	}

	if err := m.repo.SaveSession(ctx, claims); err != nil {
		return fmt.Errorf("persist admin session: %w", err)
	}

	return nil
}

func (m *Manager) deleteSessionsLocked(ctx context.Context, sessionIDs ...string) error {
	if m.repo == nil || len(sessionIDs) == 0 {
		return nil
	}

	if err := m.repo.DeleteSessions(ctx, sessionIDs); err != nil {
		return fmt.Errorf("delete persisted sessions: %w", err)
	}

	return nil
}
