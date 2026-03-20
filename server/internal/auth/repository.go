package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rayleabot/server/internal/storage"
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
	SaveSession(context.Context, Claims) error
	DeleteSessions(context.Context, []string) error
}

type SQLiteRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}

	return &SQLiteRepository{
		read:  store.Read,
		write: store.Write,
	}, nil
}

func (r *SQLiteRepository) LoadBootstrap(ctx context.Context) (*BootstrapState, error) {
	var state BootstrapState
	var initializedAt string
	err := r.read.QueryRowContext(
		ctx,
		`SELECT identifier, secret_digest, signing_key, initialized_at
		FROM auth_bootstrap_state
		WHERE singleton_id = 1`,
	).Scan(&state.Identifier, &state.SecretDigest, &state.SigningKey, &initializedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load bootstrap state: %w", err)
	}

	parsedTime, err := time.Parse(time.RFC3339Nano, initializedAt)
	if err != nil {
		return nil, fmt.Errorf("parse bootstrap initialized_at: %w", err)
	}
	state.InitializedAt = parsedTime.UTC()

	return &state, nil
}

func (r *SQLiteRepository) LoadSessions(ctx context.Context) ([]Claims, error) {
	rows, err := r.read.QueryContext(ctx, `SELECT session_id, subject, issued_at, expires_at FROM admin_sessions`)
	if err != nil {
		return nil, fmt.Errorf("query admin sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]Claims, 0)
	for rows.Next() {
		var claims Claims
		var issuedAt string
		var expiresAt string
		if err := rows.Scan(&claims.SessionID, &claims.Subject, &issuedAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan admin session: %w", err)
		}

		claims.IssuedAt, err = time.Parse(time.RFC3339Nano, issuedAt)
		if err != nil {
			return nil, fmt.Errorf("parse session issued_at: %w", err)
		}
		claims.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("parse session expires_at: %w", err)
		}
		claims.IssuedAt = claims.IssuedAt.UTC()
		claims.ExpiresAt = claims.ExpiresAt.UTC()
		sessions = append(sessions, claims)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin sessions: %w", err)
	}

	return sessions, nil
}

func (r *SQLiteRepository) SaveBootstrap(ctx context.Context, state BootstrapState, session Claims) error {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bootstrap transaction: %w", err)
	}

	var existing int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM auth_bootstrap_state WHERE singleton_id = 1`,
	).Scan(&existing); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("check bootstrap state: %w", err)
	}
	if existing > 0 {
		_ = tx.Rollback()
		return ErrBootstrapAlreadyInitialized
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO auth_bootstrap_state (singleton_id, identifier, secret_digest, signing_key, initialized_at)
		VALUES (1, ?, ?, ?, ?)`,
		state.Identifier,
		state.SecretDigest,
		state.SigningKey,
		state.InitializedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert bootstrap state: %w", err)
	}

	if err := upsertSession(ctx, tx, session); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bootstrap transaction: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) SaveSession(ctx context.Context, claims Claims) error {
	if err := upsertSession(ctx, r.write, claims); err != nil {
		return err
	}

	return nil
}

func (r *SQLiteRepository) DeleteSessions(ctx context.Context, sessionIDs []string) error {
	for _, sessionID := range sessionIDs {
		if sessionID == "" {
			continue
		}
		if _, err := r.write.ExecContext(ctx, `DELETE FROM admin_sessions WHERE session_id = ?`, sessionID); err != nil {
			return fmt.Errorf("delete session %s: %w", sessionID, err)
		}
	}

	return nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func upsertSession(ctx context.Context, executor sqlExecutor, claims Claims) error {
	if _, err := executor.ExecContext(
		ctx,
		`INSERT INTO admin_sessions (session_id, subject, issued_at, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			subject = excluded.subject,
			issued_at = excluded.issued_at,
			expires_at = excluded.expires_at`,
		claims.SessionID,
		claims.Subject,
		claims.IssuedAt.UTC().Format(time.RFC3339Nano),
		claims.ExpiresAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert session %s: %w", claims.SessionID, err)
	}

	return nil
}
