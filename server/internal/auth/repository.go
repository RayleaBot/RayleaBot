package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type BootstrapState struct {
	Identifier    string
	SecretDigest  []byte
	SigningKey    []byte
	InitializedAt time.Time
}

type Repository interface {
	LoadBootstrap(context.Context) (*BootstrapState, error)
	LoadSessions(context.Context) ([]Claims, error)
	SaveBootstrap(context.Context, BootstrapState, Claims) error
	UpdateBootstrapSecretDigest(context.Context, []byte) error
	SaveSession(context.Context, Claims) error
	DeleteSessions(context.Context, []string) error
}

type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	write  *sql.DB
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}

	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		write:  store.Write,
	}, nil
}

func (r *SQLiteRepository) LoadBootstrap(ctx context.Context) (*BootstrapState, error) {
	row, err := r.readQ.LoadBootstrap(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load bootstrap state: %w", err)
	}

	parsedTime, err := time.Parse(time.RFC3339Nano, row.InitializedAt)
	if err != nil {
		return nil, fmt.Errorf("parse bootstrap initialized_at: %w", err)
	}

	return &BootstrapState{
		Identifier:    row.Identifier,
		SecretDigest:  row.SecretDigest,
		SigningKey:    row.SigningKey,
		InitializedAt: parsedTime.UTC(),
	}, nil
}

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

func (r *SQLiteRepository) SaveBootstrap(ctx context.Context, state BootstrapState, session Claims) error {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bootstrap transaction: %w", err)
	}

	q := r.writeQ.WithTx(tx)

	existing, err := q.CountBootstrap(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("check bootstrap state: %w", err)
	}
	if existing > 0 {
		_ = tx.Rollback()
		return ErrBootstrapAlreadyInitialized
	}

	if err := q.InsertBootstrap(ctx, sqlcgen.InsertBootstrapParams{
		Identifier:    state.Identifier,
		SecretDigest:  state.SecretDigest,
		SigningKey:    state.SigningKey,
		InitializedAt: state.InitializedAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert bootstrap state: %w", err)
	}

	if err := q.UpsertSession(ctx, claimsToUpsertParams(session)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("upsert session %s: %w", session.SessionID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bootstrap transaction: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) SaveSession(ctx context.Context, claims Claims) error {
	if err := r.writeQ.UpsertSession(ctx, claimsToUpsertParams(claims)); err != nil {
		return fmt.Errorf("upsert session %s: %w", claims.SessionID, err)
	}
	return nil
}

func (r *SQLiteRepository) UpdateBootstrapSecretDigest(ctx context.Context, secretDigest []byte) error {
	affected, err := r.writeQ.UpdateBootstrapSecretDigest(ctx, secretDigest)
	if err != nil {
		return fmt.Errorf("update bootstrap secret digest: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
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
