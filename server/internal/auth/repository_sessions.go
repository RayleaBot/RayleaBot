package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) LoadSessions(ctx context.Context) ([]Claims, error) {
	rows, err := r.readQ.LoadSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("query admin sessions: %w", err)
	}

	sessions := make([]Claims, 0, len(rows))
	for _, row := range rows {
		var claims Claims
		claims.SessionID = row.SessionID
		claims.Subject = row.Subject

		claims.IssuedAt, err = time.Parse(time.RFC3339Nano, row.IssuedAt)
		if err != nil {
			return nil, fmt.Errorf("parse session issued_at: %w", err)
		}
		claims.ExpiresAt, err = time.Parse(time.RFC3339Nano, row.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("parse session expires_at: %w", err)
		}
		claims.IssuedAt = claims.IssuedAt.UTC()
		claims.ExpiresAt = claims.ExpiresAt.UTC()
		sessions = append(sessions, claims)
	}

	return sessions, nil
}

func (r *SQLiteRepository) SaveSession(ctx context.Context, claims Claims) error {
	if err := r.writeQ.UpsertSession(ctx, claimsToUpsertParams(claims)); err != nil {
		return fmt.Errorf("upsert session %s: %w", claims.SessionID, err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteSessions(ctx context.Context, sessionIDs []string) error {
	for _, sessionID := range sessionIDs {
		if sessionID == "" {
			continue
		}
		if err := r.writeQ.DeleteSession(ctx, sessionID); err != nil {
			return fmt.Errorf("delete session %s: %w", sessionID, err)
		}
	}
	return nil
}

func claimsToUpsertParams(claims Claims) sqlcgen.UpsertSessionParams {
	return sqlcgen.UpsertSessionParams{
		SessionID: claims.SessionID,
		Subject:   claims.Subject,
		IssuedAt:  claims.IssuedAt.UTC().Format(time.RFC3339Nano),
		ExpiresAt: claims.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
}
